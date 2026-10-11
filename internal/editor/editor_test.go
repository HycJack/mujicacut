package editor

import "testing"

// demo builds a project with one video and one audio track and two pool
// assets.
func demo(t *testing.T) *Project {
	t.Helper()
	p := &Project{Name: "demo", Width: 1920, Height: 1080, FPS: 30}
	p.Media = append(p.Media,
		&Media{ID: "m1", Name: "city", Kind: KindVideo, Duration: 10, Hue: 210},
		&Media{ID: "m2", Name: "bgm", Kind: KindAudio, Duration: 30, Hue: 130},
	)
	p.Tracks = append(p.Tracks,
		&Track{ID: "v1", Name: "V1", Kind: KindVideo},
		&Track{ID: "a1", Name: "A1", Kind: KindAudio},
	)
	return p
}

func mustClip(t *testing.T, p *Project, id string) *Clip {
	t.Helper()
	_, c := p.FindClip(id)
	if c == nil {
		t.Fatalf("clip %s missing", id)
	}
	return c
}

func TestAddClipPlacesWithoutOverlap(t *testing.T) {
	p := demo(t)
	a := p.AddClip("m1", "v1", 0)
	b := p.AddClip("m1", "v1", 5) // want inside a → pushed right to 10
	if a.Start != 0 || a.Duration != 10 {
		t.Fatalf("a = %+v", a)
	}
	if b.Start != 10 {
		t.Fatalf("b.Start = %v, want 10 (pushed past a)", b.Start)
	}
}

func TestMoveClipBetweenTracksAndPushRight(t *testing.T) {
	p := demo(t)
	a := p.AddClip("m1", "v1", 0) // 0..10
	b := p.AddClip("m1", "v1", 12)
	if b.Start != 12 {
		t.Fatalf("b.Start = %v, want untouched 12", b.Start)
	}
	// a fits just before b: 2..12 abuts b's start.
	p.MoveClip(a.ID, 2, "v1")
	if got := mustClip(t, p, a.ID).Start; got != 2 {
		t.Fatalf("a.Start = %v, want 2", got)
	}
	// Overlapping b pushes a past it.
	p.MoveClip(a.ID, 8, "v1")
	if got := mustClip(t, p, a.ID).Start; got != 22 {
		t.Fatalf("a.Start = %v, want pushed to 22", got)
	}
	p.MoveClip(a.ID, 0, "a1") // kind mismatch → no-op
	if tr, _ := p.FindClip(a.ID); tr.ID != "v1" {
		t.Fatalf("cross-kind move moved the clip")
	}
}

func TestTrimClampsToNeighborsAndSource(t *testing.T) {
	p := demo(t)
	a := p.AddClip("m1", "v1", 0) // 0..10
	p.AddClip("m1", "v1", 0)      // pushed to 10..20
	// Right edge: cannot pass b's start (10).
	p.TrimClip(a.ID, false, 15)
	if got := mustClip(t, p, a.ID).End(); got != 10 {
		t.Fatalf("right trim end = %v, want 10", got)
	}
	// Left edge: source starts at 0, so the in-point cannot go before 0.
	p.TrimClip(a.ID, true, -3)
	c := mustClip(t, p, a.ID)
	if c.Start != 0 || c.Offset != 0 {
		t.Fatalf("left trim = %+v", c)
	}
	// Trim right to 5, then left-trim into the source: offset follows.
	p.TrimClip(a.ID, false, 5)
	p.TrimClip(a.ID, true, 2)
	c = mustClip(t, p, a.ID)
	if c.Start != 2 || c.Duration != 3 || c.Offset != 2 {
		t.Fatalf("after both trims = %+v", c)
	}
}

func TestSplitClip(t *testing.T) {
	p := demo(t)
	a := p.AddClip("m1", "v1", 0) // 0..10
	right := p.SplitClip(a.ID, 4)
	if right == nil {
		t.Fatal("split inside the clip returned nil")
	}
	// Re-find by ID: the append in insertClip may reallocate the slice,
	// and pointers taken before it are stale.
	if left := mustClip(t, p, a.ID); left.Start != 0 || left.Duration != 4 {
		t.Fatalf("left half = %+v", left)
	}
	r := mustClip(t, p, right.ID)
	if r.Start != 4 || r.Duration != 6 || r.Offset != 4 {
		t.Fatalf("right half = %+v", r)
	}
	if p.SplitClip(a.ID, 100) != nil {
		t.Fatal("split outside the clip should return nil")
	}
}

func TestSetSpeedKeepsSourceWindow(t *testing.T) {
	p := demo(t)
	a := p.AddClip("m1", "v1", 0) // 10s at 1×
	p.SetSpeed(a.ID, 2)
	c := mustClip(t, p, a.ID)
	if c.Speed != 2 || c.Duration != 5 {
		t.Fatalf("2× = %+v, want duration 5", c)
	}
	p.SetSpeed(a.ID, 0.25) // clamped, still the same 10s of source
	c = mustClip(t, p, a.ID)
	if c.Speed != 0.25 || c.Duration != 40 {
		t.Fatalf("0.25× = %+v, want duration 40", c)
	}
}

func TestHistoryUndoRedo(t *testing.T) {
	p := demo(t)
	h := NewHistory(50)

	mutate := func(fn func(*Project)) {
		before := p.Clone()
		fn(p)
		h.Push(before)
	}
	mutate(func(p *Project) { p.AddClip("m1", "v1", 0) })
	mutate(func(p *Project) { p.AddClip("m2", "a1", 0) })
	if len(p.Tracks[0].Clips) != 1 || len(p.Tracks[1].Clips) != 1 {
		t.Fatalf("setup = %+v", p)
	}
	if !h.CanUndo() || h.CanRedo() {
		t.Fatal("history state after two edits")
	}
	p = h.Undo(p)
	if p == nil || len(p.Tracks[1].Clips) != 0 {
		t.Fatal("undo did not restore the one-clip state")
	}
	p = h.Undo(p)
	if len(p.Tracks[0].Clips) != 0 {
		t.Fatal("undo did not restore the empty state")
	}
	if h.CanUndo() {
		t.Fatal("undo past the beginning")
	}
	p = h.Redo(p)
	if len(p.Tracks[0].Clips) != 1 {
		t.Fatal("redo did not reapply the first edit")
	}
}

func TestProjectCloneAndDuration(t *testing.T) {
	p := demo(t)
	p.AddClip("m1", "v1", 0) // 0..10
	p.AddClip("m2", "a1", 0) // 0..30 — the audio sets the length
	if d := p.Duration(); d != 30 {
		t.Fatalf("duration = %v, want 30", d)
	}
	q := p.Clone()
	p.RemoveClip(p.Tracks[0].Clips[0].ID)
	if len(q.Tracks[0].Clips) != 1 {
		t.Fatal("clone shares clip storage with the original")
	}
}
