package app

// view.go is the whole window: a title bar, a media pool on the left,
// the preview player in the middle, the property inspector on the right
// and the custom-drawn timeline along the bottom.

import (
	"github.com/HycJack/MujicaUI/layout"
	"github.com/egoist/mygo/ui"
)

// view builds one frame of the editor.
func (a *app) view(c *ui.Context) {
	k := useTheme(c)
	a.clampPlayhead()
	ui.Column(c).Fill().Background(k.Background).Children(func() {
		a.titlebar(c, k)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			a.mediaPool(c, k)
			ui.Box(c).Width(1).Background(k.Border)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				a.previewPane(c, k)
			})
			ui.Box(c).Width(1).Background(k.Border)
			a.inspector(c, k)
		})
		a.timelinePanel(c, k)
	})
	a.shortcuts(c)
}

// titlebar is the top strip: the mark and name on the left; undo, redo
// and export on the right.
func (a *app) titlebar(c *ui.Context, k tokensT) {
	layout.TitleBar(c, "MujicaCut", layout.TitleBarOptions{
		Leading: func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				mark := ui.Box(c).Size(26, 26).Radius(7).Center().
					Background(k.Accent)
				mark.Children(func() {
					ui.Icon(c, iconFilm).FontSize(15).TextColor(k.OnAccent)
				})
				ui.Column(c).Gap(0).Children(func() {
					ui.Text(c, "MujicaCut").FontSize(13).Bold()
					ui.Text(c, "剪映风格 · 原生 GPU 绘制").FontSize(9).TextColor(k.TextMuted)
				})
			})
		},
		Trailing: func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				a.iconBtn(c, k, iconUndo, "撤销 (⌘Z)", !a.hist.CanUndo(), func() { a.undo() })
				a.iconBtn(c, k, iconRedo, "重做 (⇧⌘Z)", !a.hist.CanRedo(), func() { a.redo() })
				ui.Box(c).Width(1).Height(18).Background(k.Border)
				if b := ui.PrimaryButton(c, "导出 EDL"); b.Clicked() {
					a.exportProject(c)
				}
			})
		},
	})
}

// iconBtn is a small square icon button. Disabled buttons keep their
// place but take no input.
func (a *app) iconBtn(c *ui.Context, k tokensT, icon *ui.SVG, label string, disabled bool, fn func()) {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(28, 28).Radius(7).Center().
		Cursor(ui.CursorPointer).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(k.SurfaceHover)
	}
	if b.Clicked() {
		fn()
	}
	b.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(k.Text)
	})
}

// shortcuts binds the editing keys. Space plays; S splits; Delete
// removes; the arrows step a frame; ⌘Z / ⇧⌘Z walk the history.
func (a *app) shortcuts(c *ui.Context) {
	if c.Shortcut(0, ui.KeySpace) {
		a.togglePlay()
	}
	if c.Shortcut(0, ui.KeyS) {
		a.splitAtPlayhead()
	}
	if c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace) {
		a.deleteSelected()
	}
	if c.Shortcut(0, ui.KeyLeft) {
		a.nudgePlayhead(-1)
	}
	if c.Shortcut(0, ui.KeyRight) {
		a.nudgePlayhead(1)
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
		a.redo()
	}
	if c.Shortcut(ui.Cmd, ui.KeyZ) {
		a.undo()
	}
	if c.Shortcut(ui.Cmd, ui.KeyEqual) {
		a.zoom = min(a.zoom*1.25, 400)
	}
	if c.Shortcut(ui.Cmd, ui.KeyMinus) {
		a.zoom = max(a.zoom/1.25, 8)
	}
}
