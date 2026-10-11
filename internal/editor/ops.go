package editor

import "sort"

// ops.go is the editing vocabulary: place, move, trim, split, remove,
// speed and volume. Mutations keep every track's clips sorted and
// non-overlapping; user-driven operations clamp to the nearest legal
// value instead of failing.

// AddMedia adds an asset to the pool. It panics on an empty or duplicate
// ID — pool contents are program-controlled data.
func (p *Project) AddMedia(m *Media) {
	if m == nil || m.ID == "" {
		panic("editor: AddMedia needs a media with an ID")
	}
	if p.FindMedia(m.ID) != nil {
		panic("editor: duplicate media ID " + m.ID)
	}
	p.Media = append(p.Media, m)
}

// RemoveMedia drops an asset from the pool; it reports false when the
// asset is still placed on a track (the UI refuses first).
func (p *Project) RemoveMedia(id string) bool {
	for _, tr := range p.Tracks {
		for _, c := range tr.Clips {
			if c.MediaID == id {
				return false
			}
		}
	}
	for i, m := range p.Media {
		if m.ID == id {
			p.Media = append(p.Media[:i], p.Media[i+1:]...)
			return true
		}
	}
	return false
}

// AddClip places a media asset on a track at the wanted start, pushed
// right out of any overlap. It panics on unknown IDs or a video/audio
// kind mismatch — the app layer only ever passes data it read back from
// the same project.
func (p *Project) AddClip(mediaID, trackID string, start float64) *Clip {
	m := p.FindMedia(mediaID)
	if m == nil {
		panic("editor: AddClip on unknown media " + mediaID)
	}
	tr := p.Track(trackID)
	if tr == nil {
		panic("editor: AddClip on unknown track " + trackID)
	}
	if tr.Kind == KindAudio != (m.Kind == KindAudio) {
		panic("editor: AddClip kind mismatch: " + m.Kind.String() + " on " + tr.Name)
	}
	dur := m.Duration
	if m.Kind == KindImage {
		dur = StillDur
	}
	c := Clip{
		ID:       NewID("clip"),
		MediaID:  mediaID,
		Start:    placeStart(tr, start, dur, ""),
		Duration: dur,
		Speed:    1,
		Volume:   1,
	}
	insertClip(tr, c)
	return &c
}

// MoveClip moves a clip to the wanted start on the target track (same
// kind as its current one), pushed right out of any overlap. Unknown
// clip IDs are a no-op — UI state may lag an undo.
func (p *Project) MoveClip(clipID string, start float64, trackID string) {
	from, c := p.FindClip(clipID)
	if c == nil {
		return
	}
	to := p.Track(trackID)
	if to == nil || to.Kind != from.Kind {
		return
	}
	// Work on a copy: removeClip shifts the slice, so the pointer taken
	// above would land on a neighbor storage after it.
	orig := *c
	removeClip(from, clipID)
	orig.Start = placeStart(to, start, orig.Duration, "")
	insertClip(to, orig)
}

// TrimClip moves one edge of a clip to timeline time t, clamped between
// the neighboring clip, the project's minimum length and the source's
// bounds. left selects the in-point edge.
func (p *Project) TrimClip(clipID string, left bool, t float64) {
	tr, c := p.FindClip(clipID)
	if c == nil {
		return
	}
	prevEnd, nextStart := clipNeighbors(tr, clipID)
	m := p.FindMedia(c.MediaID)
	if left {
		lo := prevEnd
		if m != nil && m.Kind != KindImage {
			lo = max(lo, c.Start-c.Offset/c.Speed) // no source before zero
		}
		newStart := clamp(t, lo, c.End()-MinClipDur)
		delta := newStart - c.Start
		c.Offset += delta * c.Speed
		c.Start = newStart
		c.Duration -= delta
		return
	}
	hi := nextStart
	if m != nil && m.Kind != KindImage && m.Duration > 0 {
		hi = min(hi, c.Start+(m.Duration-c.Offset)/c.Speed)
	}
	newEnd := clamp(t, c.Start+MinClipDur, max(hi, c.Start+MinClipDur))
	c.Duration = newEnd - c.Start
}

// SplitClip cuts a clip at timeline time t and returns the new right
// half; it returns nil when t lies outside the clip's interior.
func (p *Project) SplitClip(clipID string, t float64) *Clip {
	tr, c := p.FindClip(clipID)
	if c == nil || t <= c.Start+MinClipDur || t >= c.End()-MinClipDur {
		return nil
	}
	right := *c
	right.ID = NewID("clip")
	right.Start = t
	right.Duration = c.End() - t
	right.Offset = c.Offset + (t-c.Start)*c.Speed
	c.Duration = t - c.Start
	insertClip(tr, right)
	return &right
}

// RemoveClip drops a clip from its track; unknown IDs are a no-op.
func (p *Project) RemoveClip(clipID string) {
	if tr, c := p.FindClip(clipID); c != nil {
		removeClip(tr, clipID)
	}
}

// SetSpeed rescales the clip so it keeps consuming the same source
// window: the timeline duration shrinks as the rate grows.
func (p *Project) SetSpeed(clipID string, speed float64) {
	_, c := p.FindClip(clipID)
	if c == nil {
		return
	}
	speed = clamp(speed, 0.25, 4)
	consumed := c.Duration * c.Speed
	c.Speed = speed
	c.Duration = consumed / speed
}

// SetVolume sets an audio clip's level, clamped to 0..1.5.
func (p *Project) SetVolume(clipID string, v float64) {
	_, c := p.FindClip(clipID)
	if c == nil {
		return
	}
	c.Volume = clamp(v, 0, 1.5)
}

// placeStart returns the nearest legal start at or after want for a clip
// of the given length on the track, pushing right past busy intervals.
func placeStart(tr *Track, want, dur float64, ignoreID string) float64 {
	type span struct{ s, e float64 }
	var busy []span
	for _, c := range tr.Clips {
		if c.ID == ignoreID {
			continue
		}
		busy = append(busy, span{c.Start, c.End()})
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].s < busy[j].s })
	cand := max(0, want)
	for round := 0; round <= len(busy); round++ {
		moved := false
		for _, b := range busy {
			if cand < b.e && cand+dur > b.s { // overlap: jump past it
				cand = b.e
				moved = true
				break
			}
		}
		if !moved {
			return cand
		}
	}
	return cand
}

// clipNeighbors reports the gap a clip lives in: the previous clip's end
// and the next clip's start on its track.
func clipNeighbors(tr *Track, clipID string) (prevEnd, nextStart float64) {
	var c *Clip
	for i := range tr.Clips {
		if tr.Clips[i].ID == clipID {
			c = &tr.Clips[i]
			break
		}
	}
	if c == nil {
		return 0, 1e18
	}
	prevEnd, nextStart = 0, 1e18
	for _, o := range tr.Clips {
		if o.ID == clipID {
			continue
		}
		if o.End() <= c.Start+1e-9 {
			prevEnd = max(prevEnd, o.End())
		}
		if o.Start >= c.End()-1e-9 {
			nextStart = min(nextStart, o.Start)
		}
	}
	return prevEnd, nextStart
}

func insertClip(tr *Track, c Clip) {
	tr.Clips = append(tr.Clips, c)
	sortClips(tr)
}

func removeClip(tr *Track, id string) {
	for i := range tr.Clips {
		if tr.Clips[i].ID == id {
			tr.Clips = append(tr.Clips[:i], tr.Clips[i+1:]...)
			return
		}
	}
}

func sortClips(tr *Track) {
	sort.SliceStable(tr.Clips, func(i, j int) bool { return tr.Clips[i].Start < tr.Clips[j].Start })
}

func clamp(v, lo, hi float64) float64 {
	if hi < lo {
		hi = lo
	}
	return min(max(v, lo), hi)
}
