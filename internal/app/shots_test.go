package app

// shots_test.go renders the editor's key states to PNG for the README
// and for visual audits. It is a no-op unless MYGO_UI_SHOTS names a
// directory:
//
//	MYGO_UI_SHOTS=screenshots go test ./internal/app -run TestScreenshots

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestScreenshots(t *testing.T) {
	dir := os.Getenv("MYGO_UI_SHOTS")
	if dir == "" {
		t.Skip("set MYGO_UI_SHOTS to a directory to write the UI screenshots")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(moduleRoot(), dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	shot := func(name string, a *app) {
		t.Helper()
		tt := ui.NewTester(a.view, 1360, 860)
		// Two frames: the timeline sizes its content off the viewport
		// width, which the scroll area's handle reports from frame two on.
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		if img == nil {
			t.Fatalf("%s: no frame rendered", name)
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	// The seeded sequence, mid-scrub with a clip selected: every panel
	// shows its loaded state.
	ed := newApp()
	ed.playhead = 3.2
	ed.selectClip(ed.proj.Tracks[1].Clips[0].ID)
	shot("editor-overview", ed)

	// A zoomed-in timeline with a drag mid-flight: snap guides and the
	// moving clip should be visible.
	drag := newApp()
	drag.zoom = 150
	drag.playhead = 14.6
	drag.selectClip(drag.proj.Tracks[1].Clips[1].ID)
	drag.dragStart(float32(14.5*drag.zoom), drag.laneTop(1)+20)
	drag.dragUpdate(float32(15.9*drag.zoom), drag.laneTop(2)+20)
	shot("timeline-dragging", drag)
	drag.dragFinish()

	// The empty state: no clips anywhere.
	empty := newApp()
	for _, tr := range empty.proj.Tracks {
		tr.Clips = nil
	}
	empty.selClip = ""
	shot("editor-empty", empty)
}

// moduleRoot walks up from the working directory to the directory
// holding go.mod, so a relative MYGO_UI_SHOTS lands at the repo root no
// matter which package's directory the test runs in.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
