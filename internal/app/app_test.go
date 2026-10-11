package app

import (
	"testing"

	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// TestRenderEditor is the layout smoke test: the whole desk must render
// without panicking — every icon name, component contract (clip IDs,
// scrubber bounds, list data) and layout rule is exercised here.
func TestRenderEditor(t *testing.T) {
	for _, size := range [][2]int{{1280, 820}, {1024, 680}, {1600, 1000}} {
		if img := ui.Render(newApp().view, size[0], size[1], 1); img == nil {
			t.Fatalf("render %dx%d returned nil", size[0], size[1])
		}
	}
	// Selected clip: the inspector must render the clip variant.
	sel := newApp()
	sel.selectClip(sel.proj.Tracks[1].Clips[0].ID)
	if img := ui.Render(sel.view, 1280, 820, 1); img == nil {
		t.Fatal("render with selection returned nil")
	}
	// Mid-playback: the transport's playing variant.
	play := newApp()
	play.togglePlay()
	if img := ui.Render(play.view, 1280, 820, 1); img == nil {
		t.Fatal("render while playing returned nil")
	}
	// Empty project: the drop hint variant.
	empty := newApp()
	for _, tr := range empty.proj.Tracks {
		tr.Clips = nil
	}
	empty.selClip = ""
	if img := ui.Render(empty.view, 1280, 820, 1); img == nil {
		t.Fatal("render of an empty project returned nil")
	}
}

// TestEditorTexts checks the chrome's copy lands on screen.
func TestEditorTexts(t *testing.T) {
	tt := ui.NewTester(newApp().view, 1280, 820)
	for _, want := range []string{"媒体库", "属性", "吸附", "导出 EDL", "V1 主轨", "A1 音乐"} {
		if !tt.HasText(want) {
			t.Errorf("frame is missing %q", want)
		}
	}
}

// TestSplitAndDelete walks the two destructive actions with undo. The
// seeded project's main track starts with two clips.
func TestSplitAndDelete(t *testing.T) {
	a := newApp()
	v1 := a.proj.Tracks[1]
	a.playhead = 5
	a.selectClip(v1.Clips[0].ID)
	a.splitAtPlayhead()
	if len(v1.Clips) != 3 {
		t.Fatalf("split produced %d clips, want 3", len(v1.Clips))
	}
	a.undo()
	if len(a.proj.Tracks[1].Clips) != 2 {
		t.Fatal("undo did not restore the pre-split state")
	}
	a.redo()
	if len(a.proj.Tracks[1].Clips) != 3 {
		t.Fatal("redo did not reapply the split")
	}
	// The redo split minted a fresh clip ID, so the selection is empty;
	// select the right half again and delete it.
	a.selectClip(a.proj.Tracks[1].Clips[1].ID)
	a.deleteSelected()
	if len(a.proj.Tracks[1].Clips) != 2 {
		t.Fatal("delete did not remove the selected half")
	}
}

// TestTimelineScrub selects and scrubs through the timeline logic at the
// canvas-coordinate level (the element wiring only feeds these numbers).
func TestTimelineScrub(t *testing.T) {
	a := newApp()
	// Press in the ruler (y < rulerH) scrubs to that time.
	a.dragStart(300, 10)
	a.dragFinish()
	if a.playhead != 5 {
		t.Fatalf("scrub to x=300 put the playhead at %v, want 5", a.playhead)
	}
	// Press on a clip selects it and records the grab.
	a.dragStart(90, rulerH+20) // 1.5s into the 0..12 clip on V1
	if a.selClip == "" || a.drag.clip == "" {
		t.Fatalf("press on a clip did not select it (sel=%q)", a.selClip)
	}
	if a.drag.scrub || a.drag.trimL || a.drag.trimR {
		t.Fatalf("press on the clip body misread as %+v", a.drag)
	}
	a.dragFinish()
}

// TestTimelineDragMove moves a clip within its track and across tracks.
func TestTimelineDragMove(t *testing.T) {
	a := newApp()
	clip := a.proj.Tracks[1].Clips[1]    // the 12..22 clip; city sits at 0..12
	y := a.laneTop(1) + 20               // V1's lane
	a.dragStart(float32(13.5*a.zoom), y) // grab 1.5s into the clip
	if a.drag.clip != clip.ID {
		t.Fatalf("grab landed on %q, want %q", a.drag.clip, clip.ID)
	}
	a.dragUpdate(float32(24.5*a.zoom), y) // pointer at 24.5s, want 23 → free gap
	a.dragFinish()
	if a.drag.on {
		t.Fatal("drag did not finish")
	}
	if !a.hist.CanUndo() {
		t.Fatal("a real drag left no undo snapshot")
	}
	_, c := a.proj.FindClip(clip.ID)
	if c == nil || c.Start != 23 { // pointer at 24.5s minus the 1.5s grab
		t.Fatalf("move left the clip at %+v, want start 23", c)
	}
	// Dropping it on an audio lane is refused: same-kind lanes only.
	_, c = a.proj.FindClip(clip.ID)
	before := c.Start
	a.dragStart(float32((c.Start+0.5)*a.zoom), y)
	a.dragUpdate(float32((c.Start+0.5)*a.zoom), a.laneTop(2)+20) // A1
	a.dragFinish()
	if tr, _ := a.proj.FindClip(clip.ID); tr.Kind != editor.KindVideo {
		t.Fatal("a video clip moved onto an audio track")
	}
	_, c = a.proj.FindClip(clip.ID)
	if c.Start != before {
		t.Fatalf("refused move still changed the clip: %v → %v", before, c.Start)
	}
}

// TestTimelineTrim trims both edges and checks the neighbor clamp.
func TestTimelineTrim(t *testing.T) {
	a := newApp()
	clip := a.proj.Tracks[1].Clips[0] // 0..12 at zoom 60
	// Grab the right edge and pull left.
	a.dragStart(float32(12*a.zoom)-2, a.laneTop(1)+20)
	a.dragUpdate(float32(8*a.zoom), a.laneTop(1)+20)
	a.dragFinish()
	_, c := a.proj.FindClip(clip.ID)
	if c.Duration != 8 {
		t.Fatalf("right trim left duration %v, want 8", c.Duration)
	}
	// Pull the left edge past the source start: clamped at zero offset.
	a.dragStart(float32(c.Start*a.zoom)+2, a.laneTop(1)+20)
	a.dragUpdate(float32(-30), a.laneTop(1)+20)
	a.dragFinish()
	_, c = a.proj.FindClip(clip.ID)
	if c.Start != 0 || c.Offset != 0 {
		t.Fatalf("left trim escaped the source: %+v", c)
	}
}

// TestSnap drags a clip toward a gap's edge and expects it to land
// exactly on the neighbor's end while snapping is on.
func TestSnap(t *testing.T) {
	a := newApp()
	v1 := a.proj.Tracks[1]
	v1.Clips = nil
	a.proj.AddClip("m-city", "v1", 0)      // 0..12
	B := a.proj.AddClip("m-sea", "v1", 20) // 20..30, a gap between them
	a.selectClip(B.ID)
	y := a.laneTop(1) + 20

	// Grab B 1.5s in and drop it near A's end: the snapped start is 12.
	a.dragStart(float32(21.5*a.zoom), y)
	a.dragUpdate(float32(13.53*a.zoom), y)
	a.dragFinish()
	_, b := a.proj.FindClip(B.ID)
	if b.Start != 12 {
		t.Fatalf("snap left the clip at %v, want 12", b.Start)
	}

	// Snapping off: the same gesture lands where the pointer says.
	a.snap = false
	a.dragStart(float32(13.5*a.zoom), y) // B now sits at 12..22
	a.dragUpdate(float32(14.53*a.zoom), y)
	a.dragFinish()
	_, b = a.proj.FindClip(B.ID)
	if !near(b.Start, 13.03, 1e-6) {
		t.Fatalf("drag without snapping landed at %v, want 13.03", b.Start)
	}
}

func near(got, want, tol float64) bool {
	d := got - want
	return d < tol && d > -tol
}

// TestZoomBounds keeps the zoom slider's writes inside the toolbar's
// range.
func TestZoomBounds(t *testing.T) {
	a := newApp()
	a.zoom = 3
	if a.contentWidth() < a.vpW {
		t.Fatal("content narrower than the viewport")
	}
	a.zoom = 100000
	if w := float32(a.proj.Duration()*a.zoom) + padR; a.contentWidth() > w {
		t.Fatalf("content wider than the project at zoom: %v", a.contentWidth())
	}
}

// TestTimecode sanity-checks the stamp format.
func TestTimecode(t *testing.T) {
	if got := timecode(65.5, 30); got != "01:05.15" {
		t.Fatalf("timecode(65.5) = %q, want 01:05.15", got)
	}
	if got := timecode(-1, 30); got != "00:00.00" {
		t.Fatalf("negative timecode = %q", got)
	}
}
