package app

// timeline.go is the bottom panel: the toolbar (split, delete, snap,
// zoom, timecode), the track headers, and the scrollable timeline
// canvas — one custom-drawn surface that carries the ruler, the track
// lanes, the clips and the playhead, and answers the pointer itself.

import (
	"math"

	"github.com/HycJack/MujicaUI/icons"
	"github.com/HycJack/MujicaUI/layout"
	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// Timeline geometry, in DIPs. The track headers column and the canvas
// must agree on every number here.
const (
	rulerH  = float32(28)
	laneVH  = float32(56)
	laneAH  = float32(44)
	laneGap = float32(8)
	grabPx  = float32(6) // the trim hot zone at a clip's edges
	padR    = float32(200)
)

// timelinePanel is the whole bottom region.
func (a *app) timelinePanel(c *ui.Context, k tokensT) {
	ui.Column(c).Shrink(0).Background(panelBG(k)).Children(func() {
		ui.Box(c).FillWidth().Height(1).Background(k.Border)
		a.timelineToolbar(c, k)
		ui.Row(c).AlignItems(ui.Stretch).Children(func() {
			a.trackHeaders(c, k)
			ui.Box(c).Width(1).Background(k.Border)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				a.timelineCanvas(c, k)
			})
		})
	})
}

// timelineToolbar is the strip above the timeline.
func (a *app) timelineToolbar(c *ui.Context, k tokensT) {
	ui.Row(c).FillWidth().Padding(6, 10).Gap(8).AlignItems(ui.Center).Children(func() {
		a.iconBtn(c, k, iconScissors, "在播放头分割选中片段 (S)", false, a.splitAtPlayhead)
		a.iconBtn(c, k, icons.Must("trash-2"), "删除选中片段 (Delete)", a.selClip == "", a.deleteSelected)
		ui.Box(c).Width(1).Height(16).Background(k.Border)

		// The magnetic-snap chip: tinted while snapping is on.
		b := ui.ButtonBase(c).Label("吸附到片段边缘与播放头").Tooltip("吸附到片段边缘与播放头").
			Height(28).Padding(0, 9).Radius(7).Gap(5).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		fg := k.TextMuted
		if a.snap {
			b.Background(k.Accent)
			fg = k.OnAccent
		} else if b.Hovered() {
			b.Background(k.SurfaceHover)
		}
		if b.Clicked() {
			a.snap = !a.snap
		}
		b.Children(func() {
			ui.Icon(c, iconMagnet).FontSize(13).TextColor(fg)
			ui.Text(c, "吸附").FontSize(11).Bold().TextColor(fg)
		})

		ui.Spacer(c)
		a.iconBtn(c, k, icons.Must("zoom-out"), "缩小 (⌘-)", false, func() { a.zoom = max(a.zoom/1.25, 8) })
		ui.Slider(c, &a.zoom, 8, 400).Width(120)
		a.iconBtn(c, k, icons.Must("zoom-in"), "放大 (⌘+)", false, func() { a.zoom = min(a.zoom*1.25, 400) })
		ui.Box(c).Width(1).Height(16).Background(k.Border)
		ui.Text(c, timecode(a.playhead, a.proj.FPS)).FontSize(11.5).Bold().TextColor(k.Danger)
		ui.Text(c, "/ "+timecode(a.proj.Duration(), a.proj.FPS)).FontSize(10.5).TextColor(k.TextMuted)
	})
}

// trackHeaders is the fixed left column: one header per lane, pixel-
// matched to the canvas lanes (ruler spacer, then lane heights with the
// same gap).
func (a *app) trackHeaders(c *ui.Context, k tokensT) {
	ui.Column(c).Width(116).Shrink(0).Children(func() {
		ui.Box(c).FillWidth().Height(rulerH)
		for i, tr := range a.proj.Tracks {
			h := ui.Box(c).FillWidth().Height(a.laneHeightI(i)).
				Padding(0, 8).Background(laneBG(k, tr.Kind)).Center()
			if i > 0 {
				h.Margin(laneGap, 0, 0, 0)
			}
			h.Children(func() {
				ui.Row(c).FillWidth().Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, kindIcon(tr.Kind)).FontSize(11).TextColor(k.TextMuted)
					ui.Text(c, tr.Name).FontSize(10.5).Bold().Grow(1).MinWidth(0).SingleLine()
					ui.Textf(c, "%d", len(tr.Clips)).FontSize(9).TextColor(k.TextMuted)
				})
			})
		}
	})
}

// timelineCanvas is the scrollable drawing surface. The content is sized
// off the viewport width measured on the previous frame — Resolve only
// answers after this pass built the scroll area, so the measurement lags
// one frame and converges immediately after.
func (a *app) timelineCanvas(c *ui.Context, k tokensT) {
	if a.playing {
		phX := float32(a.playhead * a.zoom)
		if phX > a.scroll.X+a.vpW-48 || phX < a.scroll.X {
			a.scroll.X = max(0, phX-a.vpW*0.25)
		}
	}

	layout.ScrollArea(c, &a.scroll, layout.ScrollAreaOptions{
		Direction: layout.ScrollHorizontal, Label: "时间轴",
	}, func() {
		inner := ui.Box(c).Size(a.contentWidth(), a.canvasH()+4)
		inner.Draw(a.drawTimeline(k))
		inner.DrawOver(a.drawOverlays(k))
		a.timelineInput(inner)
	}).Height(a.canvasH() + 4).Ref(&a.tlRef)

	if el := c.Resolve(a.tlRef); el.Valid() {
		if w := el.Bounds().W; w > 0 {
			a.vpW = w
		}
	}
}

// timelineInput is the canvas's pointer logic: press to select or scrub,
// drag to move across tracks, to trim from either edge, or to scrub.
func (a *app) timelineInput(e ui.Element) {
	px, py, _ := e.PointerPosition()
	pressed := e.Pressed()
	switch {
	case pressed && !a.drag.on:
		a.dragStart(px, py)
	case pressed && a.drag.on:
		a.dragUpdate(px, py)
	case !pressed && a.drag.on:
		a.dragFinish()
	}
}

// laneHeightI is track i's lane height.
func (a *app) laneHeightI(i int) float32 {
	if a.proj.Tracks[i].Kind == editor.KindAudio {
		return laneAH
	}
	return laneVH
}

// laneTop is track i's lane's top edge on the canvas.
func (a *app) laneTop(i int) float32 {
	y := rulerH
	for j := 0; j < i; j++ {
		y += a.laneHeightI(j) + laneGap
	}
	return y
}

// canvasH is the lanes' total height below the ruler.
func (a *app) canvasH() float32 {
	return a.laneTop(len(a.proj.Tracks)) - laneGap + 4
}

// laneFromY maps a canvas y to its track index; -1 when on no lane.
func (a *app) laneFromY(y float32) int {
	for i := range a.proj.Tracks {
		top := a.laneTop(i)
		if y >= top && y < top+a.laneHeightI(i) {
			return i
		}
	}
	return -1
}

// contentWidth is the canvas's width: the project at the current zoom
// plus trailing room, never narrower than the viewport.
func (a *app) contentWidth() float32 {
	w := float32(a.proj.Duration()*a.zoom) + padR
	return max(w, a.vpW)
}

// dragStart grabs whatever the press landed on.
func (a *app) dragStart(x, y float32) {
	a.drag = tlDrag{on: true, startX: x}
	t := max(float64(x), 0) / a.zoom
	if y < rulerH || a.laneFromY(y) < 0 {
		a.drag.scrub = true
		a.setPlayhead(t)
		return
	}
	tr := a.proj.Tracks[a.laneFromY(y)]
	for i := range tr.Clips {
		c := tr.Clips[i]
		if t < c.Start || t >= c.End() {
			continue
		}
		a.selectClip(c.ID)
		a.drag.clip = c.ID
		edge := float64(grabPx) / a.zoom
		switch {
		case t < c.Start+edge:
			a.drag.trimL = true
		case t > c.End()-edge:
			a.drag.trimR = true
		default:
			a.drag.grabT = t - c.Start
		}
		a.undoSnap = a.proj.Clone()
		return
	}
	// Empty lane: clear the selection and scrub.
	a.selectClip("")
	a.drag.scrub = true
	a.setPlayhead(t)
}

// dragUpdate applies the drag gesture live; history gets one snapshot
// from dragStart, pushed by dragFinish.
func (a *app) dragUpdate(x, y float32) {
	t := max(float64(x), 0) / a.zoom
	if x != a.drag.startX {
		a.drag.moved = true
	}
	switch {
	case a.drag.scrub:
		a.setPlayhead(t)
	case a.drag.trimL:
		a.proj.TrimClip(a.drag.clip, true, a.snapTime(t))
	case a.drag.trimR:
		a.proj.TrimClip(a.drag.clip, false, a.snapTime(t))
	default:
		tr, c := a.proj.FindClip(a.drag.clip)
		if c == nil {
			return
		}
		want := a.snapMove(t-a.drag.grabT, c.Duration)
		trackID := tr.ID
		if lane := a.laneFromY(y); lane >= 0 {
			if target := a.proj.Tracks[lane]; target.Kind == tr.Kind {
				trackID = target.ID // same-kind lanes: free track switching
			}
		}
		a.proj.MoveClip(a.drag.clip, want, trackID)
	}
}

// dragFinish commits the drag's undo snapshot when it actually moved.
func (a *app) dragFinish() {
	if a.drag.moved && a.drag.clip != "" && a.undoSnap != nil {
		a.hist.Push(a.undoSnap)
	}
	a.drag, a.undoSnap = tlDrag{}, nil
}

// setPlayhead clamps and applies a scrub.
func (a *app) setPlayhead(t float64) {
	a.playhead = min(max(t, 0), a.proj.Duration())
}

// snapTime snaps an edge time to the timeline's anchors: zero, the
// playhead and every other clip's edges.
func (a *app) snapTime(t float64) float64 {
	v, _ := a.snapTimeDist(t)
	return v
}

// snapTimeDist is snapTime with the snap's distance: math.Inf(1) when
// nothing was within tolerance, so an unpinned edge never outranks a
// pinned one.
func (a *app) snapTimeDist(t float64) (float64, float64) {
	if !a.snap {
		return t, math.Inf(1)
	}
	tol := 8 / a.zoom
	best, dist := t, math.Inf(1)
	cand := func(v float64) {
		if d := math.Abs(t - v); d <= tol && d < dist {
			best, dist = v, d
		}
	}
	cand(0)
	cand(a.playhead)
	for _, tr := range a.proj.Tracks {
		for _, c := range tr.Clips {
			if c.ID == a.drag.clip {
				continue
			}
			cand(c.Start)
			cand(c.End())
		}
	}
	return best, dist
}

// snapMove snaps a moving clip by either edge, preferring whichever edge
// actually pinned; when both pin, the closer one wins.
func (a *app) snapMove(want, dur float64) float64 {
	if !a.snap {
		return max(0, want)
	}
	s, sDist := a.snapTimeDist(want)
	e, eDist := a.snapTimeDist(want + dur)
	e -= dur
	if eDist < sDist {
		return max(0, e)
	}
	return max(0, s)
}

// dragGuides lists the snap guide lines to draw while dragging.
func (a *app) dragGuides() []float64 {
	if !a.drag.on || !a.drag.moved || a.drag.scrub {
		return nil
	}
	_, c := a.proj.FindClip(a.drag.clip)
	if c == nil {
		return nil
	}
	switch {
	case a.drag.trimL:
		return []float64{c.Start}
	case a.drag.trimR:
		return []float64{c.End()}
	default:
		return []float64{c.Start, c.End()}
	}
}
