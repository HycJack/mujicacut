package app

// preview.go is the center pane: the 16:9 program monitor and the
// transport bar. Playback renders a procedural stand-in frame per clip
// (MujicaCut ships no decoder — the frame is keyed by the clip's media
// hue and its local source time, so the picture visibly plays).

import (
	"math"
	"time"

	"github.com/HycJack/MujicaUI/core"
	"github.com/HycJack/MujicaUI/media"
	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// previewPane is the center column: stage on top, transport below.
func (a *app) previewPane(c *ui.Context, k tokensT) {
	// Drive the clock while playing: the view runs once per frame, so
	// the frame delta is the elapsed time, and AnimationFrame keeps the
	// frames coming.
	if a.playing {
		now := c.Now()
		if !a.lastTick.IsZero() {
			a.playhead += now.Sub(a.lastTick).Seconds()
			if d := a.proj.Duration(); a.playhead >= d {
				a.playhead = d
				a.playing = false
				a.lastTick = time.Time{}
			} else {
				c.AnimationFrame()
			}
		} else {
			c.AnimationFrame()
		}
		a.lastTick = now
	} else {
		a.lastTick = time.Time{}
	}

	ui.Column(c).Grow(1).MinWidth(0).Background(k.Background).Children(func() {
		a.previewStage(c, k)
		a.transport(c, k)
	})
}

// previewStage is the monitor: the program picture letterboxed inside a
// dark stage. The 16:9 rect is computed in the painter — layout-level
// aspect fitting proved fragile, and one Draw owns the whole thing.
func (a *app) previewStage(c *ui.Context, k tokensT) {
	stage := ui.Box(c).Grow(1).MinHeight(0).Background(stageBG(k))
	stage.Draw(a.drawProgramFrame(k))
	if stage.Clicked() {
		a.togglePlay()
	}
}

// transport is the player bar: frame step, play/pause, the timecode and
// a scrubber, with the sequence format on the right.
func (a *app) transport(c *ui.Context, k tokensT) {
	ui.Column(c).Shrink(0).Gap(6).Padding(8, 14, 10).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, timecode(a.playhead, a.proj.FPS)).FontSize(12).Bold().TextColor(k.Text)
			ui.Text(c, "/ "+timecode(a.proj.Duration(), a.proj.FPS)).FontSize(11).TextColor(k.TextMuted)
			ui.Spacer(c)
			ui.Textf(c, "%d×%d · %dfps", a.proj.Width, a.proj.Height, a.proj.FPS).
				FontSize(10).TextColor(k.TextMuted)
		})
		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			a.smallBtn(c, k, core.MediaPrev, "上一帧 (←)", func() { a.nudgePlayhead(-1) })
			a.playBtn(c, k)
			a.smallBtn(c, k, core.MediaNext, "下一帧 (→)", func() { a.nudgePlayhead(1) })
			ui.Spacer(c)
		})
		// The scrubber mirrors the playhead; drags write it back.
		if d := a.proj.Duration(); d > 0 {
			pos := time.Duration(min(a.playhead, d) * float64(time.Second))
			sc := media.VideoScrubber(c, &pos, media.VideoScrubberOptions{
				Duration: time.Duration(d * float64(time.Second)),
				Step:     time.Second / time.Duration(a.proj.FPS),
				Label:    "播放头",
			})
			sc.Element.FillWidth()
			if sc.Changed() {
				a.playing = false
				a.playhead = pos.Seconds()
			}
		}
	})
}

// playBtn is the round play/pause button.
func (a *app) playBtn(c *ui.Context, k tokensT) {
	label := "播放 (空格)"
	icon := core.MediaPlay
	if a.playing {
		label, icon = "暂停 (空格)", core.MediaPause
	}
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(40, 40).Radius(20).
		Center().Cursor(ui.CursorPointer).Background(k.Accent)
	if b.Clicked() {
		a.togglePlay()
	}
	b.Children(func() {
		ui.Icon(c, icon).FontSize(18).TextColor(k.OnAccent)
	})
}

// smallBtn is a ghost icon button for the transport.
func (a *app) smallBtn(c *ui.Context, k tokensT, icon *ui.SVG, label string, fn func()) {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(30, 30).Radius(15).
		Center().Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(k.SurfaceHover)
	}
	if b.Clicked() {
		fn()
	}
	b.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(k.Text)
	})
}

// drawProgramFrame paints the stand-in program picture: the largest
// 16:9 rect inside the stage, the top-most video clip under the playhead
// rendered as a procedural scene, or a drop-hint when the screen is empty.
func (a *app) drawProgramFrame(k tokensT) func(*ui.Painter, ui.Rect) {
	return func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, stageBG(k), 0)
		w, h := r.W, r.W*9/16
		if h > r.H {
			h, w = r.H, r.H*16/9
		}
		r = ui.Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
		p.Fill(r, media.VideoBlack, 4)

		clip, m := a.proj.ClipAt(a.playhead, editor.KindVideo)
		if m == nil {
			a.drawEmptyStage(p, r, k)
			return
		}
		local := (a.playhead-clip.Start)*clip.Speed + clip.Offset // source time

		// Sky: the media hue drifting slowly with source time.
		hue := m.Hue + 8*math.Sin(local*0.35)
		top := ui.Oklch(0.38, 0.11, float32(hue))
		bot := ui.Oklch(0.16, 0.06, float32(hue+40))
		p.FillGradient(r, ui.LinearGradient{From: top, To: bot, Angle: 180, Oklab: true}, 0)

		// Sun: an orbiting disc, one lap per 8 source seconds.
		cx := r.X + r.W*float32(0.5+0.30*math.Cos(local*0.785))
		cy := r.Y + r.H*float32(0.42+0.13*math.Sin(local*1.57))
		p.Fill(ui.Rect{X: cx - 26, Y: cy - 26, W: 52, H: 52},
			ui.Oklch(0.86, 0.13, float32(hue+30)), 26)

		// Hills: two parallax bands whose phase scrolls with time.
		for i, frac := range []float32{0.30, 0.18} {
			base := r.Y + r.H*(0.62+float32(i)*0.13)
			amp := r.H * frac * 0.35
			phase := local * float64(6+i*4)
			path := ui.Path{}
			path.MoveTo(r.X, base)
			for x := float32(0); x <= r.W; x += 8 {
				y := base - amp*float32(0.5+0.5*math.Sin(float64(x)/72*math.Pi+phase))
				path.LineTo(r.X+x, y)
			}
			path.LineTo(r.X+r.W, r.Y+r.H)
			path.LineTo(r.X, r.Y+r.H)
			path.Close()
			p.FillPath(&path, ui.Oklch(0.22-float32(i)*0.05, 0.07, float32(hue+90)))
		}

		// Grain: a slow dot drift keeps the picture alive frame to frame.
		dot := ui.RGB(255, 255, 255).Alpha(0.05)
		off := float32(math.Mod(local*14, 22))
		for y := r.Y + off; y < r.Y+r.H; y += 22 {
			for x := r.X + off; x < r.X+r.W; x += 22 {
				p.Fill(ui.Rect{X: x, Y: y, W: 1.6, H: 1.6}, dot, 0.8)
			}
		}

		// The HUD: clip name top-left, source timecode top-right, and
		// the stand-in notice.
		p.Clip(r, 0, func() {
			p.Fill(ui.Rect{X: r.X + 10, Y: r.Y + 10, W: 168, H: 20}, ui.RGBA(0, 0, 0, 0.5), 5)
			p.Text(r.X+18, r.Y+14.5, m.Name, 11, ui.RGB(255, 255, 255))
			tc := timecode(local, a.proj.FPS)
			w, _ := p.MeasureText(200, ui.Span{Text: tc, Size: 11})
			p.Fill(ui.Rect{X: r.X + r.W - w - 26, Y: r.Y + 10, W: w + 16, H: 20}, ui.RGBA(0, 0, 0, 0.5), 5)
			p.Text(r.X+r.W-w-18, r.Y+14.5, tc, 11, ui.RGB(255, 255, 255))
			notice := "合成预览 · 无解码器"
			nw, _ := p.MeasureText(240, ui.Span{Text: notice, Size: 9})
			p.Text(r.X+r.W-nw-12, r.Y+r.H-10, notice, 9, ui.RGB(255, 255, 255).Alpha(0.45))
		})
	}
}

// drawEmptyStage is the no-clip hint.
func (a *app) drawEmptyStage(p *ui.Painter, r ui.Rect, k tokensT) {
	msg := "把素材加到时间轴，画面会在这里播放"
	w, h := p.MeasureText(r.W, ui.Span{Text: msg, Size: 13, Weight: 700})
	p.Text(r.X+(r.W-w)/2, r.Y+r.H/2, msg, 13, k.TextMuted)
	sub := "播放的是程序合成的示意画面，不需要真实解码"
	sw, _ := p.MeasureText(r.W, ui.Span{Text: sub, Size: 10})
	p.Text(r.X+(r.W-sw)/2, r.Y+r.H/2+h+10, sub, 10, k.TextMuted.Alpha(0.7))
}
