// MujicaCut is a CapCut-style video editor built on mygo's native UI
// toolkit and the MujicaUI component library: a media pool beside a
// preview player, a property inspector, and a fully custom-drawn NLE
// timeline with drag-to-move, trim handles, splitting, magnetic
// snapping and undo/redo. No web page, no HTML, no JavaScript — the
// whole window is GPU-drawn. Playback shows a procedural stand-in frame
// per clip, so the demo runs without any real decoder.
//
// Layout of the code (two layers plus a bootstrap):
//   - internal/editor — the model layer: the project/track/clip data,
//     the editing operations (move, trim, split, snap) and the undo
//     history. No UI imports.
//   - internal/app — the UI layer: theme, layout, the media pool, the
//     preview player, the inspector and the timeline.
//   - main.go (this file) — a bootstrap that calls app.Run.
package main

import (
	"log"

	"github.com/HycJack/mujicacut/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
