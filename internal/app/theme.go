package app

// theme.go installs MujicaUI's theme (forced dark: an editor reads best
// on dark, the way every NLE ships), forwards the tokens, defines the
// few icons MujicaUI doesn't ship, and formats timecodes.

import (
	"fmt"

	"github.com/HycJack/MujicaUI/core"
	"github.com/HycJack/MujicaUI/icons"
	"github.com/HycJack/MujicaUI/theme"
	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

type tokensT = theme.Tokens

// useTheme installs the library theme in dark mode and returns the
// frame's tokens. Call once at the top of the view.
func useTheme(c *ui.Context) tokensT {
	core.Use(c, core.Settings{Mode: core.Dark})
	return core.Tokens(c)
}

// panelBG tints the side panels a step off the background.
func panelBG(k tokensT) ui.Color {
	return k.Background.Mix(k.Text, 0.04)
}

// stageBG is the preview stage: the background pushed toward black.
func stageBG(k tokensT) ui.Color {
	return k.Background.Mix(ui.RGB(0, 0, 0), 0.45)
}

// Media asset stand-in colors, derived from the asset's hue. These color
// content (thumbnails, preview frames), not the chrome.
func mediaFace(hue float64, l, c float32) ui.Color {
	return ui.Oklch(l, c, float32(hue))
}

// Lucide stroke paths for the icons MujicaUI's set doesn't carry.
var (
	iconScissors = core.MediaSVG(`<circle cx="6" cy="6" r="3"/><path d="M8.12 8.12 12 12"/><path d="M20 4 8.12 15.88"/><circle cx="6" cy="18" r="3"/><path d="M14.8 14.8 20 20"/>`)
	iconUndo     = core.MediaSVG(`<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 5.5 5.5v0a5.5 5.5 0 0 1-5.5 5.5H11"/>`)
	iconRedo     = core.MediaSVG(`<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5A5.5 5.5 0 0 0 4 14.5v0A5.5 5.5 0 0 0 9.5 20H13"/>`)
	iconFilm     = core.MediaSVG(`<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M7 3v18"/><path d="M3 7.5h4"/><path d="M3 12h18"/><path d="M3 16.5h4"/><path d="M17 3v18"/><path d="M17 7.5h4"/><path d="M17 16.5h4"/>`)
	iconMusic    = core.MediaSVG(`<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>`)
	iconMagnet   = core.MediaSVG(`<path d="m6 15-4-4 6.75-6.77a7.79 7.79 0 0 1 11 11L13 22l-4-4 6.39-6.36a2.14 2.14 0 0 0-3-3L6 15"/><path d="m5 8 4 4"/><path d="m12 15 4 4"/>`)
	iconSparkle  = core.MediaSVG(`<path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"/>`)
)

// kindIcon maps a media kind to its icon.
func kindIcon(kind editor.Kind) *ui.SVG {
	switch kind {
	case editor.KindAudio:
		return iconMusic
	case editor.KindImage:
		return icons.Must("image")
	default:
		return iconFilm
	}
}

// timecode formats t as MM:SS.FF at the project's frame rate.
func timecode(t float64, fps int) string {
	if t < 0 {
		t = 0
	}
	frames := int(t*float64(fps) + 0.5)
	ff := frames % fps
	sec := frames / fps
	return fmt.Sprintf("%02d:%02d.%02d", sec/60, sec%60, ff)
}
