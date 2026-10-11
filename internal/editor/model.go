// Package editor is MujicaCut's model layer: the project / track / clip
// data, the editing operations over it and the undo history. It imports
// no UI packages — the app layer renders it and routes input back into
// these operations.
//
// The contract, the fail-fast way: constructors and Add* panic on empty
// or duplicate IDs and on kind mismatches; user-facing operations (move,
// trim, place) never panic — they clamp to the nearest legal value.
package editor

import "sync/atomic"

// Kind is a media asset's type.
type Kind uint8

const (
	KindVideo Kind = iota
	KindAudio
	KindImage
)

func (k Kind) String() string {
	switch k {
	case KindAudio:
		return "audio"
	case KindImage:
		return "image"
	default:
		return "video"
	}
}

// Media is one asset in the project's pool. Duration is in seconds; a
// still image carries 0 and gets a fixed still duration when placed.
// Hue colors the asset's stand-in thumbnail and preview (0..360).
type Media struct {
	ID       string
	Name     string
	Kind     Kind
	Duration float64
	Hue      float64
	Path     string // empty for the built-in sample assets
}

// Clip is one placement of a media asset on a track. Start is the
// timeline position in seconds; Offset is the source in-point; Duration
// is the timeline duration (source consumed = Duration × Speed).
type Clip struct {
	ID       string
	MediaID  string
	Start    float64
	Offset   float64
	Duration float64
	Speed    float64
	Volume   float64
}

// End is the clip's timeline out-point.
func (c Clip) End() float64 { return c.Start + c.Duration }

// Track is one timeline lane. Clips are kept sorted by Start.
type Track struct {
	ID    string
	Name  string
	Kind  Kind // KindVideo or KindAudio
	Clips []Clip
}

// Project is a whole editing session.
type Project struct {
	Name   string
	Width  int
	Height int
	FPS    int
	Media  []*Media
	Tracks []*Track
}

// MinClipDur is the shortest a clip may become by trimming.
const MinClipDur = 0.1

// StillDur is the timeline duration a still image gets when placed.
const StillDur = 4.0

var idSeq atomic.Uint64

// NewID mints a unique, stable-per-run ID with the given prefix.
func NewID(prefix string) string {
	return prefix + "-" + strconvUint(idSeq.Add(1))
}

func strconvUint(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// Duration returns the project's total length: the last clip's end.
func (p *Project) Duration() float64 {
	d := 0.0
	for _, tr := range p.Tracks {
		for _, c := range tr.Clips {
			if e := c.End(); e > d {
				d = e
			}
		}
	}
	return d
}

// Track returns the track with the given ID, or nil.
func (p *Project) Track(id string) *Track {
	for _, tr := range p.Tracks {
		if tr.ID == id {
			return tr
		}
	}
	return nil
}

// Media returns the media asset with the given ID, or nil.
func (p *Project) FindMedia(id string) *Media {
	for _, m := range p.Media {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// FindClip locates a clip by ID and reports its track.
func (p *Project) FindClip(id string) (*Track, *Clip) {
	for _, tr := range p.Tracks {
		for i := range tr.Clips {
			if tr.Clips[i].ID == id {
				return tr, &tr.Clips[i]
			}
		}
	}
	return nil, nil
}

// ClipAt returns the top-most clip covering time t — tracks are ordered
// top to bottom (overlay tracks first), so the first match wins — plus
// its media.
func (p *Project) ClipAt(t float64, kind Kind) (*Clip, *Media) {
	for _, tr := range p.Tracks {
		if tr.Kind != kind {
			continue
		}
		for i := len(tr.Clips) - 1; i >= 0; i-- {
			c := tr.Clips[i]
			if t >= c.Start && t < c.End() {
				return &tr.Clips[i], p.FindMedia(c.MediaID)
			}
		}
	}
	return nil, nil
}

// Clone deep-copies the whole project, for undo snapshots.
func (p *Project) Clone() *Project {
	q := *p
	q.Media = make([]*Media, len(p.Media))
	for i, m := range p.Media {
		mc := *m
		q.Media[i] = &mc
	}
	q.Tracks = make([]*Track, len(p.Tracks))
	for i, tr := range p.Tracks {
		tc := *tr
		tc.Clips = append([]Clip(nil), tr.Clips...)
		q.Tracks[i] = &tc
	}
	return &q
}
