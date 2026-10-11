package app

// timelinedraw.go is the timeline canvas's paint work: the ruler, the
// lanes, the clips (thumbnail stripes for video, waveforms for audio)
// and the overlays (snap guides, playhead). All of it is deterministic
// from app state — painting owns nothing.

import (
	"fmt"
	"math"

	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// laneBG tints a lane by kind, barely: structure reads through.
func laneBG(k tokensT, kind editor.Kind) ui.Color {
	if kind == editor.KindAudio {
		return k.Success.Alpha(0.05)
	}
	return k.Accent.Alpha(0.05)
}

// tickStep picks the ruler's labeled step: the smallest one at least 64
// DIP apart at the current zoom.
func (a *app) tickStep() float64 {
	for _, s := range []float64{0.5, 1, 2, 5, 10, 15, 30, 60, 120, 300, 600} {
		if s*a.zoom >= 64 {
			return s
		}
	}
	return 1200
}

// drawTimeline paints the whole canvas under the pointer: ruler, lanes,
// clips.
func (a *app) drawTimeline(k tokensT) func(*ui.Painter, ui.Rect) {
	return func(p *ui.Painter, r ui.Rect) {
		// The ruler: labels and ticks across the top.
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: rulerH}, k.Surface, 0)
		p.Line(r.X, r.Y+rulerH-0.5, r.X+r.W, r.Y+rulerH-0.5, 1, k.Border)
		step := a.tickStep()
		for t := 0.0; ; t += step {
			x := r.X + float32(t*a.zoom)
			if x > r.X+r.W+1 {
				break
			}
			p.Line(x, r.Y+rulerH-8, x, r.Y+rulerH-1, 1, k.TextMuted.Alpha(0.6))
			p.Text(x+4, r.Y+rulerH-10, timecode(t, a.proj.FPS), 9, k.TextMuted)
		}

		// The lanes and their clips.
		for i, tr := range a.proj.Tracks {
			lane := ui.Rect{X: r.X, Y: r.Y + a.laneTop(i), W: r.W, H: a.laneHeightI(i)}
			p.Fill(lane, laneBG(k, tr.Kind), 0)
			p.Line(lane.X, lane.Y+lane.H-0.5, lane.X+lane.W, lane.Y+lane.H-0.5, 1, k.Border.Alpha(0.5))
			for _, c := range tr.Clips {
				a.drawClip(p, k, lane, c)
			}
		}

		// Past the project's end, dim the world so the sequence's edge
		// reads at a glance.
		endX := r.X + float32(a.proj.Duration()*a.zoom)
		if endX < r.X+r.W {
			p.Fill(ui.Rect{X: endX, Y: r.Y + rulerH, W: r.X + r.W - endX, H: r.H - rulerH},
				ui.RGB(0, 0, 0).Alpha(0.25), 0)
		}
	}
}

// drawClip paints one clip block: its face, video thumbnail stripes or
// an audio waveform, the name plate, and the trim grips when selected.
func (a *app) drawClip(p *ui.Painter, k tokensT, lane ui.Rect, c editor.Clip) {
	m := a.proj.FindMedia(c.MediaID)
	if m == nil {
		return
	}
	selected := c.ID == a.selClip
	body := ui.Rect{
		X: lane.X + float32(c.Start*a.zoom) + 1,
		Y: lane.Y + 2,
		W: max(float32(c.Duration*a.zoom)-2, 6),
		H: lane.H - 4,
	}
	if body.W < 2 {
		return
	}

	if m.Kind == editor.KindAudio {
		p.Fill(body, k.Success.Alpha(0.16), 6)
		cy := body.Y + body.H/2
		seed := m.Hue*0.017 + c.Offset
		for x := body.X + 3; x < body.X+body.W-3; x += 3 {
			src := (float64(x-body.X) / a.zoom) * c.Speed
			h := max(float32(waveHeight(seed, src))*body.H*0.42, 1.5)
			p.Fill(ui.Rect{X: x, Y: cy - h, W: 1.8, H: h * 2}, k.Success.Alpha(0.9), 0.9)
		}
	} else {
		// Thumbnail stripes: the stand-in strip, one frame of it per
		// 14 DIP, keyed by source time so trims visibly shift it.
		for x := body.X + 2; x < body.X+body.W-2; x += 14 {
			src := c.Offset + (float64(x-body.X)/a.zoom)*c.Speed
			col := mediaFace(m.Hue+8*math.Sin(src*0.9), float32(0.36+0.05*math.Sin(src*1.3)), 0.09)
			p.Fill(ui.Rect{X: x, Y: body.Y + 2, W: min(12, body.X+body.W-2-x), H: body.H - 4}, col, 3)
		}
	}

	// The frame and the name plate.
	border, bw := k.ControlBorder, float32(1)
	if selected {
		border, bw = k.Accent, 2
	}
	p.Stroke(body, border, 6, bw)
	label := m.Name
	if clip := c; clip.Speed != 1 {
		label += "  ·  " + speedLabel(clip.Speed)
	}
	lw, _ := p.MeasureText(body.W, ui.Span{Text: label, Size: 10})
	plate := ui.Rect{X: body.X + 4, Y: body.Y + 4, W: min(lw+10, body.W-8), H: 16}
	p.Fill(plate, ui.RGBA(0, 0, 0, 0.8), 4)
	p.Clip(plate, 4, func() {
		p.Text(plate.X+5, plate.Y+3, label, 10, ui.RGB(255, 255, 255))
	})

	// Trim grips on the selected clip's edges.
	if selected {
		grip := k.Accent
		p.Fill(ui.Rect{X: body.X + 2, Y: body.Y + body.H/2 - 8, W: 3, H: 16}, grip, 1.5)
		p.Fill(ui.Rect{X: body.X + body.W - 5, Y: body.Y + body.H/2 - 8, W: 3, H: 16}, grip, 1.5)
	}
}

// drawOverlays paints above everything: snap guides while dragging and
// the playhead.
func (a *app) drawOverlays(k tokensT) func(*ui.Painter, ui.Rect) {
	return func(p *ui.Painter, r ui.Rect) {
		for _, g := range a.dragGuides() {
			x := r.X + float32(g*a.zoom)
			p.Line(x, r.Y, x, r.Y+r.H, 1, k.Accent.Alpha(0.6))
		}
		x := r.X + float32(a.playhead*a.zoom)
		if x < r.X-1 || x > r.X+r.W+1 {
			return
		}
		p.Line(x, r.Y, x, r.Y+r.H, 1.5, k.Danger)
		tr := ui.Path{}
		tr.MoveTo(x-5, r.Y+2)
		tr.LineTo(x+5, r.Y+2)
		tr.LineTo(x, r.Y+12)
		tr.Close()
		p.FillPath(&tr, k.Danger)
	}
}

// waveHeight is the stand-in waveform's amplitude in 0..1: a sine mix
// seeded per asset, stable across frames and trims.
func waveHeight(seed, x float64) float64 {
	v := 0.52 + 0.28*math.Sin(x*1.7+seed) + 0.16*math.Sin(x*4.3+seed*2.7) +
		0.06*math.Sin(x*11+seed*5.1)
	return min(max(v, 0.06), 1)
}

func speedLabel(s float64) string {
	if s == math.Trunc(s) {
		return fmt.Sprintf("%d×", int64(s))
	}
	return fmt.Sprintf("%.2f×", s)
}
