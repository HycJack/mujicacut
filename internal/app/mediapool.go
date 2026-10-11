package app

// mediapool.go is the left panel: the asset pool. Samples render
// procedural stand-in thumbnails; a path can be typed in to import a
// real file (ffprobe reads its duration when installed).

import (
	"github.com/HycJack/MujicaUI/icons"
	"github.com/HycJack/MujicaUI/layout"
	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// mediaPool is the left column.
func (a *app) mediaPool(c *ui.Context, k tokensT) {
	ui.Column(c).Width(246).Shrink(0).Background(panelBG(k)).Children(func() {
		ui.Row(c).FillWidth().Padding(10, 12, 4).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Must("layout-grid")).FontSize(13).TextColor(k.TextMuted)
			ui.Text(c, "媒体库").FontSize(12.5).Bold().Grow(1)
			ui.Textf(c, "%d 个素材", len(a.proj.Media)).FontSize(9.5).TextColor(k.TextMuted)
		})

		// Import by path: no native file dialog is exposed upstream
		// (docs/upstream.md), so the demo types a path instead.
		ui.Row(c).FillWidth().Padding(4, 10, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, &a.pathInput).Placeholder("输入媒体文件路径…").
				FontSize(11).Grow(1).MinWidth(0).
				Tooltip("支持视频/音频/图片；时长由 ffprobe 探测，未安装则按 15 秒")
			if b := ui.Button(c, "导入"); b.Clicked() {
				a.importPath(c)
			}
		})

		layout.ScrollArea(c, nil, layout.ScrollAreaOptions{Label: "媒体库"}, func() {
			ui.Column(c).FillWidth().Padding(0, 10, 10).Gap(8).Children(func() {
				for _, m := range a.proj.Media {
					a.mediaCard(c, k, m)
				}
			})
		}).Grow(1).MinHeight(0)
	})
}

// mediaCard is one asset: the stand-in thumbnail, the name and duration,
// and a plus button that drops it on the timeline at the playhead.
func (a *app) mediaCard(c *ui.Context, k tokensT, m *editor.Media) {
	card := ui.Column(c).FillWidth().Radius(8).Border(1, k.Border).
		Background(k.Surface).Gap(0)
	if card.Hovered() {
		card.Background(k.SurfaceHover)
	}
	card.Children(func() {
		thumb := ui.Box(c).FillWidth().Height(62).Radius(8, 8, 0, 0).Clip()
		thumb.Draw(drawMediaThumb(m))
		thumb.Children(func() {
			kindTag := ui.Row(c).Margin(6).Padding(2, 6).Radius(5).
				Background(ui.RGBA(0, 0, 0, 0.55)).Gap(4).AlignItems(ui.Center)
			kindTag.Children(func() {
				ui.Icon(c, kindIcon(m.Kind)).FontSize(9).TextColor(ui.RGB(255, 255, 255))
				ui.Text(c, kindLabel(m)).FontSize(8.5).TextColor(ui.RGB(255, 255, 255))
			})
		})
		ui.Row(c).FillWidth().Padding(6, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, m.Name).FontSize(11).Bold().SingleLine()
				ui.Text(c, mediaDurLabel(m)).FontSize(9).TextColor(k.TextMuted)
			})
			plus := ui.ButtonBase(c).Label("添加到播放头处").Tooltip("添加到播放头处").
				Size(24, 24).Radius(6).Center().Cursor(ui.CursorPointer).
				Background(k.Accent)
			if plus.Clicked() {
				a.addAtPlayhead(m)
			}
			plus.Children(func() {
				ui.Icon(c, icons.Must("plus")).FontSize(14).TextColor(k.OnAccent)
			})
		})
	})
}

func kindLabel(m *editor.Media) string {
	switch m.Kind {
	case editor.KindAudio:
		return "音频"
	case editor.KindImage:
		return "图片"
	default:
		return "视频"
	}
}

func mediaDurLabel(m *editor.Media) string {
	if m.Kind == editor.KindImage {
		return "静态图 · 入轨 4 秒"
	}
	return "时长 " + timecode(m.Duration, 30)
}

// drawMediaThumb paints the asset's stand-in thumbnail: a hue-tinted
// gradient with a film-strip, wave bars or a mountain glyph by kind.
func drawMediaThumb(m *editor.Media) func(*ui.Painter, ui.Rect) {
	return func(p *ui.Painter, r ui.Rect) {
		base := mediaFace(m.Hue, 0.42, 0.10)
		dark := mediaFace(m.Hue, 0.22, 0.12)
		p.FillGradient(r, ui.LinearGradient{From: base, To: dark, Angle: 115, Oklab: true}, 0)

		switch m.Kind {
		case editor.KindAudio:
			// Wave bars across the middle.
			cy := r.Y + r.H/2
			bw := float32(3)
			gap := float32(3)
			for i, x := 0, r.X+8; x+bw <= r.X+r.W-8; i, x = i+1, x+bw+gap {
				h := float32(waveHeight(m.Hue, float64(i))) * (r.H * 0.55)
				p.Fill(ui.Rect{X: x, Y: cy - h/2, W: bw, H: max(h, 2)},
					ui.RGB(255, 255, 255).Alpha(0.75), bw/2)
			}
		case editor.KindImage:
			// A horizon and two peaks, the picture glyph.
			cy := r.Y + r.H*0.72
			ground := ui.RGB(255, 255, 255).Alpha(0.30)
			path := ui.Path{}
			path.MoveTo(r.X+8, cy)
			path.LineTo(r.X+r.W*0.34, r.Y+r.H*0.34)
			path.LineTo(r.X+r.W*0.52, cy)
			path.Close()
			p.FillPath(&path, ground)
			path2 := ui.Path{}
			path2.MoveTo(r.X+r.W*0.42, cy)
			path2.LineTo(r.X+r.W*0.66, r.Y+r.H*0.44)
			path2.LineTo(r.X+r.W-8, cy)
			path2.Close()
			p.FillPath(&path2, ground)
			p.Fill(ui.Rect{X: r.X + r.W - 22, Y: r.Y + 10, W: 10, H: 10}, ui.RGB(255, 255, 255).Alpha(0.5), 5)
		default:
			// Film-strip perforations top and bottom.
			perf := ui.RGB(255, 255, 255).Alpha(0.28)
			for x := r.X + 6; x+6 <= r.X+r.W-6; x += 12 {
				p.Fill(ui.Rect{X: x, Y: r.Y + 5, W: 6, H: 4}, perf, 1.5)
				p.Fill(ui.Rect{X: x, Y: r.Y + r.H - 9, W: 6, H: 4}, perf, 1.5)
			}
			// A centered play glyph, the video mark.
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			tr := ui.Path{}
			s := float32(11)
			tr.MoveTo(cx-s/2, cy-s*0.62)
			tr.LineTo(cx-s/2, cy+s*0.62)
			tr.LineTo(cx+s*0.8, cy)
			tr.Close()
			p.FillPath(&tr, ui.RGB(255, 255, 255).Alpha(0.85))
		}
		// The bottom scrim keeps the card readable on light hues.
		scrim := ui.Rect{X: r.X, Y: r.Y + r.H - 14, W: r.W, H: 14}
		p.FillGradient(scrim, ui.LinearGradient{From: ui.RGB(0, 0, 0).Alpha(0), To: ui.RGB(0, 0, 0).Alpha(0.35), Angle: 180}, 0)
	}
}
