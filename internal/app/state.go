package app

// state.go is MujicaCut's app state and the actions over it. The view is
// a pure function of this struct; actions mutate it, the next frame
// renders the result.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HycJack/mujicacut/internal/editor"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// app is the whole editor state.
type app struct {
	proj *editor.Project
	hist *editor.History

	// transport
	playing  bool
	playhead float64 // seconds
	lastTick time.Time

	// timeline
	zoom    float64 // pixels per second
	scroll  ui.ScrollState
	snap    bool
	vpW     float32 // the timeline viewport's width, as of the last frame
	tlRef   ui.Handle
	selClip string

	// timeline drag, in canvas content coordinates
	drag     tlDrag
	undoSnap *editor.Project // pre-drag snapshot, committed on release

	// media pool
	pathInput string

	// inspector mirrors the selected clip's editable numbers, reloaded
	// when the selection changes so typing isn't clobbered mid-edit.
	inspLoaded string
	insp       struct {
		start, dur, vol, speed float64
	}
}

// tlDrag is the live timeline gesture.
type tlDrag struct {
	on     bool
	scrub  bool
	trimL  bool
	trimR  bool
	clip   string
	grabT  float64 // seconds between the pointer and the clip's start
	startX float32
	moved  bool
}

// Run boots the editor: build the seeded demo project, open the window
// and pump the event loop. It returns when the window closes.
func Run() error {
	a := newApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "MujicaCut — video editor",
			Width:     1360,
			Height:    860,
			MinWidth:  1080,
			MinHeight: 680,
			StateKey:  "mujicacut",
			Content:   ui.View(a.view),
		})
	})
	return mygo.App.Run()
}

func newApp() *app {
	a := &app{
		proj: seedProject(),
		hist: editor.NewHistory(100),
		zoom: 60,
		snap: true,
	}
	return a
}

// sampleMedia is the built-in asset pool. Every asset renders a
// procedural stand-in picture keyed by its hue, so the demo needs no
// decoder and no files on disk.
func sampleMedia() []*editor.Media {
	m := func(id, name string, kind editor.Kind, dur, hue float64) *editor.Media {
		return &editor.Media{ID: id, Name: name, Kind: kind, Duration: dur, Hue: hue}
	}
	return []*editor.Media{
		m("m-city", "城市延时", editor.KindVideo, 12, 215),
		m("m-sea", "海边日落", editor.KindVideo, 10, 25),
		m("m-street", "街头漫步", editor.KindVideo, 8, 285),
		m("m-coffee", "咖啡特写", editor.KindVideo, 6, 150),
		m("m-bgm", "轻快 BGM", editor.KindAudio, 30, 130),
		m("m-beat", "节奏鼓点", editor.KindAudio, 18, 45),
		m("m-title", "片头标题", editor.KindImage, 0, 330),
	}
}

// seedProject lays out a small finished-feeling sequence, so the first
// frame already looks like a project mid-edit.
func seedProject() *editor.Project {
	p := &editor.Project{Name: "未命名项目", Width: 1920, Height: 1080, FPS: 30}
	p.Media = sampleMedia()
	p.Tracks = []*editor.Track{
		{ID: "v2", Name: "V2 覆盖", Kind: editor.KindVideo},
		{ID: "v1", Name: "V1 主轨", Kind: editor.KindVideo},
		{ID: "a1", Name: "A1 音乐", Kind: editor.KindAudio},
		{ID: "a2", Name: "A2 音效", Kind: editor.KindAudio},
	}
	p.AddClip("m-city", "v1", 0)
	p.AddClip("m-sea", "v1", 12)
	p.AddClip("m-title", "v2", 1)
	p.AddClip("m-bgm", "a1", 0)
	return p
}

// mutate snapshots the project, applies fn and records the snapshot for
// undo. One call is one undo step.
func (a *app) mutate(fn func(p *editor.Project)) {
	before := a.proj.Clone()
	fn(a.proj)
	a.hist.Push(before)
}

// clampPlayhead keeps the playhead inside the project as edits change
// its length.
func (a *app) clampPlayhead() {
	if d := a.proj.Duration(); a.playhead > d {
		a.playhead = d
	}
	if a.playhead < 0 {
		a.playhead = 0
	}
}

// togglePlay starts or stops playback.
func (a *app) togglePlay() {
	a.playing = !a.playing
	a.lastTick = time.Time{}
	if d := a.proj.Duration(); a.playing && a.playhead >= d {
		a.playhead = 0 // replay from the top, the way players do
	}
}

// nudgePlayhead steps one frame left or right.
func (a *app) nudgePlayhead(frames float64) {
	a.playing = false
	a.playhead += frames / float64(a.proj.FPS)
	a.clampPlayhead()
}

// addAtPlayhead places an asset on its kind's default track (the main
// video track, or the first audio track) at the playhead and selects it.
func (a *app) addAtPlayhead(m *editor.Media) {
	trackID := ""
	for _, tr := range a.proj.Tracks {
		if m.Kind == editor.KindAudio && tr.Kind == editor.KindAudio {
			trackID = tr.ID // first audio track
			break
		}
		if m.Kind != editor.KindAudio && tr.Kind == editor.KindVideo {
			trackID = tr.ID // keep going: the last video track is the main one
		}
	}
	if trackID == "" {
		return
	}
	var placed *editor.Clip
	a.mutate(func(p *editor.Project) {
		placed = p.AddClip(m.ID, trackID, a.playhead)
	})
	a.selClip = placed.ID
	a.loadInsp()
}

// splitAtPlayhead splits the selected clip — or, with no selection, the
// clip under the playhead — at the playhead.
func (a *app) splitAtPlayhead() {
	id := a.selClip
	if id == "" {
		if c, _ := a.proj.ClipAt(a.playhead, editor.KindVideo); c != nil {
			id = c.ID
		} else if c, _ := a.proj.ClipAt(a.playhead, editor.KindAudio); c != nil {
			id = c.ID
		}
	}
	if id == "" {
		return
	}
	var made *editor.Clip
	a.mutate(func(p *editor.Project) { made = p.SplitClip(id, a.playhead) })
	if made != nil {
		a.selClip = made.ID
		a.loadInsp()
	}
}

// deleteSelected removes the selected clip.
func (a *app) deleteSelected() {
	if a.selClip == "" {
		return
	}
	a.mutate(func(p *editor.Project) { p.RemoveClip(a.selClip) })
	a.selClip = ""
	a.inspLoaded = ""
	a.clampPlayhead()
}

// selectClip switches the selection and reloads the inspector mirror.
func (a *app) selectClip(id string) {
	a.selClip = id
	a.loadInsp()
}

// loadInsp refreshes the inspector's bound fields from the selected
// clip, but only when the selection changed — mid-edit refreshes would
// clobber the number inputs.
func (a *app) loadInsp() {
	tr, c := a.proj.FindClip(a.selClip)
	if c == nil {
		a.selClip, a.inspLoaded = "", ""
		return
	}
	_ = tr
	if a.inspLoaded == a.selClip {
		return
	}
	a.inspLoaded = a.selClip
	a.insp.start, a.insp.dur = c.Start, c.Duration
	a.insp.vol, a.insp.speed = c.Volume, c.Speed
}

// undo / redo restore or reapply the last snapshot.
func (a *app) undo() {
	if p := a.hist.Undo(a.proj); p != nil {
		a.proj = p
		a.selectClip(a.selClip)
		a.clampPlayhead()
	}
}

func (a *app) redo() {
	if p := a.hist.Redo(a.proj); p != nil {
		a.proj = p
		a.selectClip(a.selClip)
		a.clampPlayhead()
	}
}

// exportProject writes the project JSON (the edit decision list) into
// the user's Downloads folder and toasts the path.
func (a *app) exportProject(c *ui.Context) {
	home, err := os.UserHomeDir()
	if err != nil {
		c.Toast("导出失败：" + err.Error())
		return
	}
	name := strings.TrimSpace(a.proj.Name)
	if name == "" {
		name = "mujicacut"
	}
	path := filepath.Join(home, "Downloads", name+".mujicacut.json")
	b, err := json.MarshalIndent(a.proj, "", "  ")
	if err == nil {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		c.Toast("导出失败：" + err.Error())
		return
	}
	c.Toast("已导出 EDL：" + path)
}

// importPath probes a media file (ffprobe when available, a 15-second
// guess otherwise) and adds it to the pool. Synchronous on purpose: a
// demo shouldn't juggle goroutine state.
func (a *app) importPath(c *ui.Context) {
	path := strings.TrimSpace(a.pathInput)
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		c.Toast("读不到文件：" + path)
		return
	}
	if info.IsDir() {
		c.Toast("这是一个目录，需要文件路径")
		return
	}
	kind := kindFromExt(path)
	dur := 15.0
	out, err := exec.CommandContext(context.Background(), "ffprobe", "-v", "error",
		"-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err == nil {
		if d := strings.TrimSpace(string(out)); d != "" {
			if v, err := parseSeconds(d); err == nil && v > 0 {
				dur = v
			}
		}
	}
	if kind == editor.KindImage {
		dur = 0
	}
	hue := float64(len(a.proj.Media) * 67 % 360)
	a.mutate(func(p *editor.Project) {
		p.AddMedia(&editor.Media{
			ID: editor.NewID("media"), Name: filepath.Base(path),
			Kind: kind, Duration: dur, Hue: hue, Path: path,
		})
	})
	a.pathInput = ""
	c.Toast("已导入：" + filepath.Base(path))
}

func kindFromExt(path string) editor.Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".wav", ".aac", ".m4a", ".flac", ".ogg":
		return editor.KindAudio
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".heic":
		return editor.KindImage
	default:
		return editor.KindVideo
	}
}

func parseSeconds(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscan(s, &v)
	return v, err
}
