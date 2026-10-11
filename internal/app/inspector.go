package app

// inspector.go is the right panel: the selected clip's properties, or
// the project's summary when nothing is selected.

import (
	"github.com/HycJack/MujicaUI/icons"
	"github.com/HycJack/MujicaUI/layout"
	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo/ui"
)

// inspector is the right column.
func (a *app) inspector(c *ui.Context, k tokensT) {
	a.loadInsp()
	ui.Column(c).Width(268).Shrink(0).Background(panelBG(k)).Children(func() {
		ui.Row(c).FillWidth().Padding(10, 12, 8).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Must("sliders-horizontal")).FontSize(13).TextColor(k.TextMuted)
			ui.Text(c, "属性").FontSize(12.5).Bold()
		})
		layout.ScrollArea(c, nil, layout.ScrollAreaOptions{Label: "属性面板"}, func() {
			ui.Column(c).FillWidth().Padding(0, 10, 10).Gap(10).Children(func() {
				if _, clip := a.proj.FindClip(a.selClip); clip != nil {
					a.clipProps(c, k)
				} else {
					a.projectProps(c, k)
				}
			})
		}).Grow(1).MinHeight(0)
	})
}

// propCard is the inspector's card shell.
func propCard(c *ui.Context, k tokensT, title string, fn func()) {
	ui.Column(c).FillWidth().Padding(10).Gap(8).Radius(8).
		Background(k.Surface).Border(1, k.Border).Children(func() {
		ui.Text(c, title).FontWeight(700).FontSize(10).TextColor(k.TextMuted)
		fn()
	})
}

// propRow is one label + control row.
func propRow(c *ui.Context, k tokensT, label string, fn func()) {
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(k.TextMuted).Width(52).Shrink(0)
		fn()
	})
}

// clipProps shows the selected clip's editable properties. The number
// fields bind to the inspector mirror; changes are diffed against the
// project, committed through the matching editor operation, and the
// mirror is read back — mutations replace the whole project, so the
// clip pointer is only good until the first one lands.
func (a *app) clipProps(c *ui.Context, k tokensT) {
	tr, clip := a.proj.FindClip(a.selClip)
	if clip == nil {
		return
	}
	m := a.proj.FindMedia(clip.MediaID)
	id, trackID := clip.ID, tr.ID
	startChanged := numberDiffers(a.insp.start, clip.Start)
	durChanged := numberDiffers(a.insp.dur, clip.Duration)
	volChanged := numberDiffers(a.insp.vol, clip.Volume)
	speedChanged := numberDiffers(a.insp.speed, clip.Speed)

	if startChanged {
		s := a.insp.start
		a.mutate(func(p *editor.Project) { p.MoveClip(id, s, trackID) })
	}
	if durChanged {
		e := a.insp.start + a.insp.dur
		a.mutate(func(p *editor.Project) { p.TrimClip(id, false, e) })
	}
	if speedChanged {
		s := a.insp.speed
		a.mutate(func(p *editor.Project) { p.SetSpeed(id, s) })
	}
	if volChanged {
		v := a.insp.vol
		a.mutate(func(p *editor.Project) { p.SetVolume(id, v) })
	}
	if startChanged || durChanged || speedChanged {
		a.insp.start = mustClipStart(a.proj, id)
		a.insp.dur = mustClipDur(a.proj, id)
		a.insp.speed = mustClipSpeed(a.proj, id)
	}

	propCard(c, k, "片段", func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			badge := ui.Box(c).Size(34, 34).Radius(7).Center()
			badge.Background(mediaFace(m.Hue, 0.5, 0.10))
			badge.Children(func() {
				ui.Icon(c, kindIcon(m.Kind)).FontSize(16).TextColor(ui.RGB(255, 255, 255))
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Text(c, m.Name).FontSize(12).Bold().SingleLine()
				ui.Text(c, kindLabel(m)+" · "+trackName(a.proj, trackID)).FontSize(9.5).TextColor(k.TextMuted)
			})
		})
	})

	propCard(c, k, "时间", func() {
		propRow(c, k, "起点", func() {
			ui.NumberInput(c, &a.insp.start, 0, 1e6, 0.1).Grow(1).MinWidth(0).FontSize(11)
		})
		propRow(c, k, "时长", func() {
			ui.NumberInput(c, &a.insp.dur, editor.MinClipDur, 1e6, 0.1).Grow(1).MinWidth(0).FontSize(11)
		})
	})

	if m.Kind == editor.KindAudio {
		propCard(c, k, "声音", func() {
			propRow(c, k, "音量", func() {
				ui.Slider(c, &a.insp.vol, 0, 1.5).Grow(1).MinWidth(0)
				ui.Textf(c, "%.0f%%", a.insp.vol*100).FontSize(10).TextColor(k.TextMuted).Width(34)
			})
		})
	}

	propCard(c, k, "变速", func() {
		propRow(c, k, "速度", func() {
			ui.StepSlider(c, &a.insp.speed, 0.25, 4, 0.25).Grow(1).MinWidth(0)
			ui.Textf(c, "%.2f×", a.insp.speed).FontSize(10).TextColor(k.TextMuted).Width(40)
		})
		ui.Text(c, "变速保持消耗同一段源素材：越快占轨道越短").FontSize(9).TextColor(k.TextMuted)
	})

	ui.Row(c).FillWidth().Gap(8).Children(func() {
		if b := ui.Button(c, "在播放头分割"); b.Clicked() {
			a.splitAtPlayhead()
		}
		b := ui.ButtonBase(c).Label("删除片段").Padding(6, 12).Radius(7).
			Background(k.DangerFill).Border(1, k.Danger).Cursor(ui.CursorPointer).Grow(1)
		if b.Clicked() {
			a.deleteSelected()
		}
		b.Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Center().Children(func() {
				ui.Icon(c, icons.Must("trash-2")).FontSize(12).TextColor(k.OnDanger)
				ui.Text(c, "删除片段").FontSize(11).Bold().TextColor(k.OnDanger)
			})
		})
	})
}

func trackName(p *editor.Project, id string) string {
	if tr := p.Track(id); tr != nil {
		return tr.Name
	}
	return "?"
}

// projectProps is the no-selection fallback: the sequence's summary.
func (a *app) projectProps(c *ui.Context, k tokensT) {
	p := a.proj
	propCard(c, k, "序列", func() {
		ui.Text(c, p.Name).FontSize(13).Bold()
		propRow(c, k, "分辨率", func() {
			ui.Textf(c, "%d × %d", p.Width, p.Height).FontSize(11)
		})
		propRow(c, k, "帧率", func() {
			ui.Textf(c, "%d fps", p.FPS).FontSize(11)
		})
		propRow(c, k, "时长", func() {
			ui.Text(c, timecode(p.Duration(), p.FPS)).FontSize(11)
		})
	})

	propCard(c, k, "轨道", func() {
		for _, tr := range p.Tracks {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, kindIcon(tr.Kind)).FontSize(11).TextColor(k.TextMuted)
				ui.Text(c, tr.Name).FontSize(11).Grow(1)
				ui.Textf(c, "%d 个片段", len(tr.Clips)).FontSize(9.5).TextColor(k.TextMuted)
			})
		}
	})

	propCard(c, k, "帮助", func() {
		for _, tip := range [][2]string{
			{"空格", "播放 / 暂停"},
			{"S", "在播放头分割选中片段"},
			{"Delete", "删除选中片段"},
			{"← / →", "逐帧移动播放头"},
			{"拖拽片段", "移动或跨轨"},
			{"拖拽边缘", "修剪入点 / 出点"},
		} {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, tip[0]).FontSize(10).Bold().TextColor(k.AccentText).Width(64).Shrink(0)
				ui.Text(c, tip[1]).FontSize(10).TextColor(k.TextMuted).Grow(1)
			})
		}
	})
}

func numberDiffers(a, b float64) bool {
	d := a - b
	return d > 1e-6 || d < -1e-6
}

func mustClipStart(p *editor.Project, id string) float64 {
	_, c := p.FindClip(id)
	if c == nil {
		return 0
	}
	return c.Start
}

func mustClipDur(p *editor.Project, id string) float64 {
	_, c := p.FindClip(id)
	if c == nil {
		return 0
	}
	return c.Duration
}

func mustClipSpeed(p *editor.Project, id string) float64 {
	_, c := p.FindClip(id)
	if c == nil {
		return 1
	}
	return c.Speed
}
