package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed web/*
var hybridFrontend embed.FS

var tasksChanged = mygo.NewEvent[taskSnapshot]("lab:tasks-changed")

type taskEntry struct {
	ID int `json:"id"`
	task
}

type taskSnapshot struct {
	Revision uint64      `json:"revision"`
	Tasks    []taskEntry `json:"tasks"`
}

// The bridge invokes methods concurrently. All model reads, mutations and
// callbacks are serialized with native UI on the application's main thread.
type TasksService struct {
	m                         *model
	run                       func(func())
	changed                   func(string)
	open                      func(int)
	applyDock                 func(bool) error
	revision                  uint64
	origin                    string
	wake                      *time.Timer
	schedule                  func(time.Duration, func())
	notices                   *nativeReminders
	nextReminder              time.Time
	notificationStatus        string
	notificationAuthorization string
	notificationDelivered     int
	export                    *nativeExport
	quickShortcut             *nativeQuickShortcut
	login                     *nativeLogin
}

func (s *TasksService) snapshot() taskSnapshot {
	out := taskSnapshot{Revision: s.revision, Tasks: make([]taskEntry, 0, s.m.totalCount())}
	for _, i := range s.m.orderedIndices() {
		t := s.m.Tasks[i]
		if !t.Deleted {
			out.Tasks = append(out.Tasks, taskEntry{ID: i + 1, task: t})
		}
	}
	return out
}

func (s *TasksService) List() (out taskSnapshot) {
	s.run(func() { out = s.snapshot() })
	return
}

func validTitle(title string) (string, error) {
	return validTitleLimit(title, 120)
}
func validModelTitle(m *model, title string) (string, error) {
	limit := 120
	if m.UITheme != "" {
		limit = 500
	}
	return validTitleLimit(title, limit)
}
func validTitleLimit(title string, limit int) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("请填写任务标题。")
	}
	if utf8.RuneCountInString(title) > limit {
		return "", fmt.Errorf("标题最多 %d 个字。", limit)
	}
	return title, nil
}

func validPriority(priority int) bool { return priority == 0 || priority == 2 || priority == 3 }

func (s *TasksService) mutate(event string, fn func() error) (out taskSnapshot, err error) {
	s.run(func() { out, err = s.mutateOnMain(event, fn) })
	return
}

// Already on the serial UI executor; do not acquire it again.
func (s *TasksService) mutateOnMain(event string, fn func() error) (taskSnapshot, error) {
	if s.origin != "" {
		event = strings.Replace(event, "web-", s.origin+"-", 1)
	}
	if s.m.Dragging {
		return taskSnapshot{}, errors.New("请先结束侧栏拖动。")
	}
	if err := s.m.commit(fn); err != nil {
		return taskSnapshot{}, err
	}
	s.revision++
	s.scheduleExpiry()
	if s.notices != nil {
		s.notices.request()
	}
	if s.changed != nil {
		s.changed(event)
	}
	return s.snapshot(), nil
}

func (s *TasksService) Add(title string, priority int) (taskSnapshot, error) {
	return s.AddDetails(title, "从任务窗口添加。", priority)
}

func (s *TasksService) AddDetails(title, note string, priority int) (taskSnapshot, error) {
	return s.AddWithPin(title, note, priority, true)
}

func (s *TasksService) AddWithPin(title, note string, priority int, pinned bool) (taskSnapshot, error) {
	return s.addWithSchedule(title, note, priority, pinned, nil)
}
func (s *TasksService) addWithSchedule(title, note string, priority int, pinned bool, schedule *taskSchedule) (taskSnapshot, error) {
	return s.mutate("web-add", func() error {
		title, err := validModelTitle(s.m, title)
		if err != nil {
			return err
		}
		if !validPriority(priority) {
			return errors.New("请选择普通、重要或紧急。")
		}
		if s.m.UITheme == "" && pinned && s.m.sidebarCount() >= s.m.sidebarCapacity() {
			return errors.New("侧栏已满，请先完成一项任务。")
		}
		if utf8.RuneCountInString(note) > 16000 {
			return errors.New("备注最多 16000 个字。")
		}
		next := task{Title: title, Note: note, Priority: priority, Version: 1, Unpinned: !pinned}
		if schedule != nil {
			if err := schedule.apply(&next); err != nil {
				return err
			}
		}
		s.m.Tasks = append(s.m.Tasks, next)
		return nil
	})
}

func (s *TasksService) editable(id int, version uint64) (*task, error) {
	if id < 1 || id > len(s.m.Tasks) || s.m.Tasks[id-1].Deleted {
		return nil, errors.New("任务不存在。")
	}
	t := &s.m.Tasks[id-1]
	if t.Version != version {
		return nil, errors.New("任务已更新，请重新编辑。")
	}
	return t, nil
}

func (s *TasksService) Update(id int, title string, priority int, version uint64) (taskSnapshot, error) {
	return s.update(id, title, nil, priority, version, nil)
}

func (s *TasksService) UpdateDetails(id int, title, note string, priority int, version uint64) (taskSnapshot, error) {
	return s.update(id, title, &note, priority, version, nil)
}

func (s *TasksService) update(id int, title string, note *string, priority int, version uint64, schedule *taskSchedule) (taskSnapshot, error) {
	return s.mutate("web-update", func() error {
		title, err := validModelTitle(s.m, title)
		if err != nil {
			return err
		}
		if !validPriority(priority) {
			return errors.New("请选择普通、重要或紧急。")
		}
		if note != nil && utf8.RuneCountInString(*note) > 16000 {
			return errors.New("备注最多 16000 个字。")
		}
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		if schedule != nil {
			if err := schedule.apply(t); err != nil {
				return err
			}
		}
		t.Title, t.Priority = title, priority
		if note != nil {
			t.Note = *note
		}
		t.Version++
		return nil
	})
}

func (s *TasksService) SetDone(id int, done bool, version uint64) (taskSnapshot, error) {
	return s.mutate("web-done", func() error {
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		if s.m.UITheme == "" && !done && t.Done && !t.Unpinned && s.m.sidebarCount() >= s.m.sidebarCapacity() {
			return errors.New("侧栏已满，请先完成一项任务。")
		}
		if done {
			s.m.recordCompletion(id - 1)
		} else if s.m.UndoID == id {
			s.m.UndoID = 0
			s.m.UndoUntil = 0
		}
		if !done {
			t.CompletedAt = 0
		}
		t.Done = done
		t.Version++
		if done && s.m.Opened == id-1 {
			s.m.close()
		}
		return nil
	})
}

func (s *TasksService) SetPinned(id int, pinned bool, version uint64) (taskSnapshot, error) {
	return s.mutate("web-pin", func() error {
		t, err := s.editable(id, version)
		if err != nil {
			return err
		}
		if s.m.UITheme == "" && pinned && t.Unpinned && !t.Done && s.m.sidebarCount() >= s.m.sidebarCapacity() {
			return errors.New("侧栏已满，请先完成一项任务。")
		}
		t.Unpinned = !pinned
		t.Version++
		if !pinned && s.m.Opened == id-1 {
			s.m.close()
		}
		return nil
	})
}

func (s *TasksService) Open(id int) (err error) {
	s.run(func() {
		if id < 1 || id > len(s.m.Tasks) || !s.m.eligible(id-1) || s.m.Quiet || s.m.Arranging {
			err = errors.New("任务不存在或已完成。")
			return
		}
		if s.m.Editing || s.m.Dragging {
			err = errors.New("请先结束卡片编辑或侧栏拖动。")
			return
		}
		s.m.open(id - 1)
		if s.open != nil {
			s.open(id - 1)
		}
	})
	return
}

type hybridApp struct {
	service           *TasksService
	window            *mygo.Window
	bounds            mygo.Rectangle
	onState           func(string)
	quitting          bool
	editCard          func()
	editItem          *mygo.MenuItem
	native            bool
	view              *nativeTasksView
	quick             *nativeQuickAdd
	tray              *mygo.Tray
	quietItem         *mygo.MenuItem
	captureQuickFocus func() quickFocusToken
	prepareQuickAdd   func()
}

func newNativeApp(m *model) *hybridApp {
	if m.UITheme == "" {
		m.UITheme, m.Side, m.Offset = "mac", "right", .35
	}
	h := &hybridApp{native: true, service: &TasksService{m: m, run: mygo.RunOnMain, origin: "native-main"}}
	h.service.login = &nativeLogin{backend: newLoginBackend()}
	h.service.enableExpiry()
	h.service.enableNotifications(h)
	h.captureQuickFocus = captureQuickForeground
	h.service.export = newNativeExport(h.service, func() {
		if !h.quitting {
			h.publish()
		}
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { h.quitting = true })
	return h
}

func (h *hybridApp) mainEvent(suffix string) string {
	if h.native {
		return "native-main-" + suffix
	}
	return "web-main-" + suffix
}

func newHybridApp(m *model) *hybridApp {
	frontend, err := fs.Sub(hybridFrontend, "web")
	if err != nil {
		panic(err)
	}
	mygo.SetFrontend(frontend)
	h := &hybridApp{service: &TasksService{m: m, run: mygo.RunOnMain}}
	mygo.Bind(h.service)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { h.quitting = true })
	return h
}

func (h *hybridApp) show() {
	if h.quitting {
		return
	}
	if h.window != nil {
		h.window.Show()
		h.window.Focus()
		return
	}
	opts := mygo.WindowOptions{Title: "Sidelet · 任务", Width: 760, Height: 600, MinWidth: 560, MinHeight: 460, BackgroundColor: "#f7f8f4"}
	if h.native {
		opts.Title = "Sidelet"
		// Wails sizes the formal task window by its content area. MyGo's
		// default includes the title bar, shrinking the page by that height.
		opts.UseContentSize = true
		opts.Width, opts.Height, opts.MinWidth, opts.MinHeight = 1120, 800, 820, 600
		h.view = newNativeTasksView(h.service)
		h.view.onHide = func() {
			if h.window != nil {
				h.window.Hide()
			}
		}
		h.view.onQuickAdd = h.showQuickAdd
		h.view.onExport = func(format string) { h.service.export.start(format, h.window) }
		h.view.onThemeChanged = func(id string) {
			if id == "graphite" {
				mygo.Theme.SetSource(mygo.ThemeDark)
			} else {
				mygo.Theme.SetSource(mygo.ThemeLight)
			}
			if h.window != nil {
				_ = h.window.SetBackgroundColor(productionDesign.Themes[id]["app-bg"])
			}
		}
		h.view.visual = webTheme(h.service.m.UITheme)
		opts.BackgroundColor = productionDesign.Themes[h.service.m.UITheme]["app-bg"]
		h.view.onThemeChanged(h.service.m.UITheme)
		opts.Content = ui.View(h.view.render)
	} else {
		opts.URL = "/"
		opts.Page = mygo.PageOptions{DevTools: mygo.DevToolsDisabled}
	}
	w := mygo.NewWindow(opts)
	h.window = w
	if h.native {
		cancelOrder := func() {
			if h.view != nil {
				h.view.order.Epoch++
				w.Invalidate()
			}
		}
		w.OnBlur(cancelOrder)
		w.OnResize(cancelOrder)
	}
	if h.bounds.Width > 0 {
		w.SetBounds(h.bounds)
	}
	w.OnClose(func(*mygo.CloseEvent) { h.bounds = w.Bounds() })
	w.OnClosed(func() {
		if h.window == w {
			h.window = nil
			h.view = nil
		}
		if h.onState != nil {
			h.onState(h.mainEvent("closed"))
		}
	})
	if !h.native {
		w.Page().OnDOMReady(func() {
			h.publish()
			if h.onState != nil {
				h.onState("web-main-ready")
			}
		})
	}
	if h.onState != nil {
		h.onState(h.mainEvent("opened"))
	}
}

func (h *hybridApp) publish() {
	if h.window != nil {
		if h.native {
			h.view.reconcile()
			h.window.Invalidate()
			if h.quick != nil && h.quick.window != nil {
				h.quick.window.Invalidate()
			}
			return
		}
		if err := tasksChanged.Emit(h.window, h.service.snapshot()); err != nil {
			log.Print(err)
		}
	}
}

func (h *hybridApp) nativeChanged(event string) {
	if h.quietItem != nil {
		label := "安静模式"
		if h.service.m.Quiet {
			label = "恢复显示"
		}
		h.quietItem.SetLabel(label)
	}
	if slices.Contains([]string{"save", "complete"}, event) {
		h.service.revision++
		h.service.scheduleExpiry()
		if h.service.notices != nil {
			h.service.notices.request()
		}
		h.publish()
	}
}

func (h *hybridApp) installMenu() {
	h.editItem = &mygo.MenuItem{Label: "编辑当前卡片", Accelerator: "CmdOrCtrl+E", Disabled: true, Click: func(*mygo.MenuItem, *mygo.Window) {
		if h.editCard != nil {
			h.editCard()
		}
	}}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "打开任务窗口", Accelerator: "CmdOrCtrl+1", Click: func(*mygo.MenuItem, *mygo.Window) { h.show() }},
			{Label: "快速添加", Click: func(*mygo.MenuItem, *mygo.Window) {
				if h.native {
					h.showQuickAdd()
				}
			}, Disabled: !h.native},
			h.editItem,
			mygo.Separator(), {Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu}, {Role: mygo.RoleViewMenu}, {Role: mygo.RoleWindowMenu},
	}))
}
