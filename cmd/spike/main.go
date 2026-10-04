//go:build windows || darwin

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"sidelet/frontend"
	"sidelet/internal/platform"
	"sidelet/internal/settings"
	"sidelet/internal/spike"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

type message struct {
	Settings       settings.Value     `json:"settings"`
	Type           string             `json:"type"`
	RequestID      string             `json:"requestId"`
	Action         spike.Action       `json:"action"`
	Rects          []platform.Rect    `json:"rects"`
	Anchor         platform.Rect      `json:"anchor"`
	ViewportWidth  float64            `json:"viewportWidth"`
	ViewportHeight float64            `json:"viewportHeight"`
	Mode           string             `json:"mode"`
	IDs            []int64            `json:"ids"`
	Stack          todo.EdgeStack     `json:"stack"`
	Side           string             `json:"side"`
	Offset         float64            `json:"offset"`
	ItemHeight     int                `json:"itemHeight"`
	Window         application.Window `json:"-"`
	Metric         json.RawMessage    `json:"metric"`
	Inside         bool               `json:"inside"`
	Enabled        bool               `json:"enabled"`
	Revision       uint64             `json:"revision"`
	X              float64            `json:"x"`
	Y              float64            `json:"y"`
	Label          string             `json:"label"`
	Source         string             `json:"source"`
}
type overlay struct {
	window         *application.WebviewWindow
	native         *platform.Window
	displayID      string
	side           string
	offset         float64
	index          int
	stackID        int64
	mode           string
	editReturnMode string
	itemHeight     int
	regionsLogged  bool
}
type controller struct {
	preferences           *settings.Store
	loginAvailable        bool
	app                   *application.App
	control               *application.WebviewWindow
	stacks                []*overlay
	quick                 *overlay
	snapshot              spike.Snapshot
	queue                 *messageQueue
	done                  chan struct{}
	previous              platform.FocusToken
	quiet                 bool
	arranging             bool
	drag                  *stackDrag
	fullscreen            bool
	watchCleanup          func()
	showControl           bool
	tracePointer          bool
	quickSession          spike.QuickSession
	quickTimer            *time.Timer
	fullscreenTimer       *time.Timer
	pointers              map[string]bool
	memoryDir             string
	memoryLabel           string
	memoryViews           map[string]bool
	interactionTest       bool
	traceFocus            bool
	store                 *storage.Store
	cleanupTimer          *time.Timer
	reminderTimer         *time.Timer
	reminderPrefix        string
	notificationDelivered int
	notificationStatus    map[string]any
}

func main() {
	version := flag.Bool("version", false, "print the application version and exit")
	two := flag.Bool("two-stacks", false, "split the eight fixtures across two Stack windows")
	show := flag.Bool("main", false, "show the main window on launch")
	fixtures := flag.Bool("spike", false, "use eight disposable in-memory tasks instead of the user database")
	dataDir := flag.String("data-dir", "", "override the normal Sidelet application data directory")
	logPath := flag.String("log-file", "", "write diagnostic logs to this file")
	tracePointer := flag.Bool("trace-pointer", false, "log overlay hit transitions for native interaction testing")
	memoryDir := flag.String("memory-dir", "", "opt-in local heap/native/frontend snapshots; request via request.txt")
	interactionTest := flag.Bool("interaction-test", false, "opt-in Mac fixture keyboard entry and focus boundary logs; does not synthesize keys")
	traceFocus := flag.Bool("trace-focus", false, "log focus boundaries without enabling the fixture keyboard entry")
	flag.Parse()
	if *version {
		fmt.Printf("Sidelet %s (build %s)\n", appVersion, appBuild)
		return
	}
	if !*fixtures && *two {
		log.Fatal("-two-stacks is a fixture option; use -spike -two-stacks")
	}
	if *fixtures && *dataDir != "" {
		log.Fatal("-spike does not open a database; omit -data-dir")
	}
	memoryQuery := ""
	if *fixtures {
		memoryQuery = "&spike=1"
	}
	if *memoryDir != "" {
		if err := os.MkdirAll(*memoryDir, 0700); err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(*memoryDir + "/request.txt"); !os.IsNotExist(err) {
			log.Fatal("memory request.txt must not exist on startup")
		}
		runtime.MemProfileRate = 64 * 1024
		memoryQuery += "&memory=1"
	}
	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.SetOutput(file)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("Sidelet %s (build %s) / Local Preview; spike=%t platform=%s pid=%d", appVersion, appBuild, *fixtures, runtime.GOOS, os.Getpid())
	if info, ok := debug.ReadBuildInfo(); ok {
		log.Printf("toolchain=%s", info.GoVersion)
		for _, dep := range info.Deps {
			if dep.Path == "github.com/wailsapp/wails/v3" {
				log.Printf("wails=%s", dep.Version)
			}
		}
	}
	assets, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	c := &controller{snapshot: spike.New(time.Now()), queue: newMessageQueue(), done: make(chan struct{}), showControl: *show, tracePointer: *tracePointer, pointers: map[string]bool{}}
	c.memoryDir = *memoryDir
	c.interactionTest = *interactionTest
	c.traceFocus = *traceFocus || *interactionTest
	var singleInstance *application.SingleInstanceOptions
	if !*fixtures {
		if *dataDir == "" {
			*dataDir, err = storage.DefaultDirectory()
			if err != nil {
				log.Fatal(err)
			}
		}
		*dataDir, err = filepath.Abs(*dataDir)
		if err != nil {
			log.Fatal(err)
		}
		profileID := sha256.Sum256([]byte(*dataDir))
		c.reminderPrefix = fmt.Sprintf("sidelet.%x.", profileID[:8])
		singleInstance = &application.SingleInstanceOptions{UniqueID: fmt.Sprintf("io.sidelet.profile.%x", profileID[:16]),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { c.post(message{Type: "show-control"}) }}
	}
	c.app = application.New(application.Options{
		Name:           "Sidelet",
		Description:    "Desktop task companion",
		Assets:         application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Windows:        application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Mac:            application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		SingleInstance: singleInstance,
		RawMessageHandler: func(w application.Window, raw string, origin *application.OriginInfo) {
			if len(raw) > 65536 {
				return
			}
			var m message
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				log.Printf("invalid bridge message: %v", err)
				return
			}
			m.Window = w
			c.post(m)
		},
		OnShutdown: func() { close(c.done) },
	})
	windowTitle := "Sidelet · Phase 0"
	if !*fixtures {
		c.preferences = settings.Open(*dataDir)
		defaultDir, defaultErr := storage.DefaultDirectory()
		c.loginAvailable = runtime.GOOS == "darwin" && defaultErr == nil && filepath.Clean(defaultDir) == *dataDir
		c.store, err = storage.OpenWithDefaults(*dataDir, time.Now(), c.preferences.Value.Edge.DefaultSide, c.preferences.Value.Edge.DefaultDensity)
		if err != nil {
			log.Fatal(err)
		}
		defer c.store.Close()
		c.snapshot, err = c.store.Snapshot(time.Now())
		if err != nil {
			log.Fatal(err)
		}
		c.showControl = *show || c.preferences.Value.Startup.ShowMainWindow || (!c.preferences.Exists && len(c.snapshot.Todos) == 0)
		log.Printf("startup show-main=%t settings-exists=%t", c.showControl, c.preferences.Exists)
		windowTitle = "Sidelet · 我的任务"
		log.Printf("storage=sqlite schema=%d tasks=%d path=%s", storage.SchemaVersion, len(c.snapshot.Todos), filepath.Join(*dataDir, "sidelet.sqlite3"))
	}
	c.control = c.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "control", Title: windowTitle, Width: 1120, Height: 800, MinWidth: 820, MinHeight: 600, URL: "/?view=control&platform=" + runtime.GOOS + memoryQuery, Hidden: true, BackgroundColour: application.NewRGB(247, 246, 243)})
	c.control.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) { e.Cancel(); c.control.Hide() })
	count := 1
	if *two {
		count = 2
	}
	if c.store != nil {
		count = len(c.snapshot.Stacks)
	}
	for i := 0; i < count; i++ {
		name := "stack-0"
		url := "/?view=stack&index=0"
		if i > 0 {
			name = fmt.Sprintf("stack-%d", i)
			url = fmt.Sprintf("/?view=stack&index=%d", i)
		}
		w := c.app.Window.NewWithOptions(overlayOptions(name, url+"&platform="+runtime.GOOS+memoryQuery))
		o := &overlay{window: w, side: "right", offset: 0.35, index: i, mode: "Passive", itemHeight: 44}
		if c.store != nil {
			saved := c.snapshot.Stacks[i]
			o.stackID = saved.ID
			o.displayID = saved.DisplayID
			o.side = saved.Side
			o.offset = saved.Offset
			o.itemHeight = storage.ItemHeight(saved.Density)
		}
		c.stacks = append(c.stacks, o)
	}
	c.quick = &overlay{window: c.app.Window.NewWithOptions(overlayOptions("quick", "/?view=quick&platform="+runtime.GOOS+memoryQuery)), mode: "Passive"}
	menu := c.app.NewMenu()
	menuLabel := "我的任务"
	if *fixtures {
		menuLabel = "显示验证面板"
	}
	menu.Add(menuLabel).OnClick(func(*application.Context) { c.post(message{Type: "show-control"}) })
	menu.Add("键盘操作 · " + shortcutLabel()).OnClick(func(*application.Context) { c.post(message{Type: "keyboard", Source: "tray-menu"}) })
	menu.Add("安静模式 / 恢复").OnClick(func(*application.Context) { c.post(message{Type: "quiet"}) })
	if !*fixtures {
		menu.Add("设置…").OnClick(func(*application.Context) { c.post(message{Type: "show-settings"}) })
		menu.Add("整理桌面 / 完成整理").OnClick(func(*application.Context) { c.post(message{Type: "toggle-arrange"}) })
	}
	if *fixtures {
		menu.Add("重置假数据").OnClick(func(*application.Context) { c.post(message{Type: "action", Action: spike.Action{Type: "reset"}}) })
	}
	menu.AddSeparator()
	menu.Add("退出 Sidelet").OnClick(func(*application.Context) { c.post(message{Type: "quit"}) })
	c.app.SystemTray.New().SetIcon(trayIcon()).SetMenu(menu).OnClick(func() { c.post(message{Type: "show-control"}) })
	go c.loop()
	go c.watchMemoryRequests()
	if err := c.app.Run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func (c *controller) post(m message) {
	c.queue.post(m, c.done)
}
func (c *controller) loop() {
	for {
		m, ok := c.queue.next(c.done)
		if !ok {
			return
		}
		c.process(m)
	}
}
func (c *controller) find(w application.Window) *overlay {
	if w == nil {
		return nil
	}
	for _, o := range c.stacks {
		if o.window.Name() == w.Name() {
			return o
		}
	}
	if c.quick.window.Name() == w.Name() {
		return c.quick
	}
	return nil
}
func (c *controller) bind(o *overlay) error {
	if o.native != nil {
		return nil
	}
	if c.memoryDir != "" {
		platform.EnableMemoryDiagnostics()
	}
	w, err := platform.Bind(o.window.NativeWindow(), platform.Callbacks{
		Hotkey:          func() { c.post(message{Type: "keyboard", Source: "global-shortcut"}) },
		TestKeyboard:    func() { c.post(message{Type: "keyboard", Source: "fixture-request"}) },
		Changed:         func() { c.post(message{Type: "displays"}) },
		Blur:            func() { c.post(message{Type: "blur"}) },
		CheckFullscreen: func() { c.post(message{Type: "fullscreen-check"}) },
		Pointer: func(x, y float64, inside bool) {
			c.post(message{Type: "pointer", Window: o.window, X: x, Y: y, Inside: inside})
		},
	})
	if err != nil {
		return err
	}
	o.native = w
	if err := w.SetRegions(nil, 312, 600); err != nil {
		return err
	}
	if o == c.stacks[0] {
		if c.store != nil {
			platform.StartNotifications(func(kind, id string) { c.post(message{Type: "notification-event", Source: kind, Label: id}) })
			c.post(messageForReminders())
		}
		if err := registerShortcut(c, w); err != nil {
			log.Print(err)
			c.app.Event.Emit("spike:error", err.Error())
		}
		cleanup, err := platform.WatchForeground(func() { c.post(message{Type: "foreground"}) })
		if err != nil {
			return err
		}
		c.watchCleanup = cleanup
		if c.interactionTest {
			if err := w.EnableInteractionTest(); err != nil {
				return err
			}
			log.Print("interaction-test enabled; fixture requests are not global key delivery")
		}
	}
	return nil
}
func (c *controller) handle(m message) error {
	o := c.find(m.Window)
	switch m.Type {
	case "stack-drag-start":
		return c.beginStackDrag(m)
	case "stack-drag-preview":
		return c.previewStackDrag(m)
	case "stack-drag-cancel":
		if c.drag != nil && c.drag.revision == m.Revision && c.drag.overlay.window == m.Window {
			c.cancelStackDrag()
		}
	case "notification-event":
		c.notificationEvent(m)
	case "memory-view":
		return c.memoryView(m)
	case "ready":
		if c.preferences != nil {
			c.post(message{Type: "settings-refresh"})
		}
		if c.notificationStatus != nil && m.Window != nil {
			m.Window.EmitEvent("reminder:status", c.notificationStatus)
		}
		if o != nil {
			if err := c.bind(o); err != nil {
				return err
			}
			if o != c.quick {
				if err := c.layout(); err != nil {
					return err
				}
			}
		}
		if m.Window != nil {
			m.Window.EmitEvent("spike:state", c.snapshot.Copy())
			m.Window.EmitEvent("spike:measure")
			c.emitPresentation()
		}
		if m.Window == c.control && c.showControl {
			c.control.Show()
			c.control.Focus()
		}
		if m.Window == c.control && c.store != nil {
			c.control.EmitEvent("spike:config", c.stackConfig(c.stacks[0]))
		}
	case "regions":
		if o != nil && o.native != nil && len(m.Rects) <= 32 {
			if err := o.native.SetRegions(m.Rects, m.ViewportWidth, m.ViewportHeight); err != nil {
				return err
			}
			if c.tracePointer && o.regionsLogged {
				// Acceptance needs the installed geometry after expansion/collapse,
				// not a stale startup rectangle. Normal runs do not emit this trace.
				logNativeWindow(o)
			}
			if len(m.Rects) > 0 && !o.regionsLogged {
				o.regionsLogged = true
				log.Printf("regions window=%s count=%d viewport=%.0fx%.0f", o.window.Name(), len(m.Rects), m.ViewportWidth, m.ViewportHeight)
				logNativeWindow(o)
			}
		}
	case "metric":
		if o != nil {
			log.Printf("hover window=%s metric=%s", o.window.Name(), m.Metric)
			if c.tracePointer {
				logNativeWindow(o)
			}
		}
	case "action":
		if o != nil {
			c.refreshFullscreen()
			if c.quiet || c.fullscreen {
				c.acknowledge(m, errors.New("desktop input is unavailable"))
				return nil
			}
		}
		spike.Apply(&c.snapshot, m.Action, time.Now())
		c.app.Event.Emit("spike:state", c.snapshot.Copy())
		if m.Action.Type == "select" && c.quickSession.Open {
			c.quickSession.TodoID = m.Action.ID
			c.emitPresentation()
		}
		if m.Action.Type == "complete" || m.Action.Type == "snooze" {
			if o == c.quick || (c.quickSession.Open && c.quickSession.TodoID == m.Action.ID) {
				c.hideQuick(true)
			}
		}
		if m.Action.Type == "reset" {
			c.hideQuick(true)
		}
		c.acknowledge(m, nil)
	case "quick", "overflow":
		if c.arranging {
			return nil
		}
		c.refreshFullscreen()
		if c.quiet || c.fullscreen {
			return nil
		}
		source := o
		if source == nil || source == c.quick {
			source = c.stacks[0]
		}
		if source.native == nil || c.quick.native == nil {
			return nil
		}
		spike.Apply(&c.snapshot, spike.Action{Type: "select", ID: m.Action.ID}, time.Now())
		if m.Type == "overflow" {
			for _, id := range m.IDs {
				for _, todo := range c.snapshot.Todos {
					if todo.ID == id {
						c.snapshot.OverflowIDs = append(c.snapshot.OverflowIDs, id)
					}
				}
			}
		}
		c.app.Event.Emit("spike:state", c.snapshot.Copy())
		if err := c.quick.native.PlaceQuick(source.native, m.Anchor, source.side); err != nil {
			return err
		}
		c.quickSession.Begin(source.window.Name(), m.Action.ID)
		c.quickSession.Presence(source.window.Name(), c.pointers[source.window.Name()], time.Now())
		c.quick.window.EmitEvent("spike:config", c.stackConfig(source))
		c.emitPresentation()
		c.quick.native.ShowInactive()
		if c.tracePointer {
			logNativeWindow(source)
			logNativeWindow(c.quick)
		}
		if m.Mode == "Editing" || source.mode == "KeyboardActive" {
			return c.enterMode(c.quick, m.Mode)
		}
		c.scheduleQuickClose()
	case "presence":
		if o != nil {
			c.pointers[o.window.Name()] = m.Inside
			c.quickSession.Presence(o.window.Name(), m.Inside, time.Now())
			c.scheduleQuickClose()
		}
	case "pointer":
		if o != nil {
			if m.Inside {
				c.refreshFullscreen()
				if c.fullscreen {
					return nil
				}
			}
			if c.tracePointer {
				log.Printf("pointer window=%s inside=%t x=%.1f y=%.1f", o.window.Name(), m.Inside, m.X, m.Y)
			}
			c.pointers[o.window.Name()] = m.Inside
			c.quickSession.Presence(o.window.Name(), m.Inside, time.Now())
			c.scheduleQuickClose()
			o.window.EmitEvent("spike:pointer", map[string]any{"x": m.X, "y": m.Y, "inside": m.Inside})
		}
	case "quick-expire":
		if !c.interactionActive() && c.quickSession.Expired(m.Revision, time.Now()) {
			c.hideQuick(false)
		}
	case "mode":
		if o != nil {
			if o.mode == "Editing" && m.Mode == "Passive" && o.editReturnMode == "KeyboardActive" {
				return c.enterMode(o, "KeyboardActive")
			}
			if m.Mode == "Passive" {
				c.exitModes(true)
				c.scheduleQuickClose()
			} else {
				return c.enterMode(o, m.Mode)
			}
		}
	case "keyboard":
		source := m.Source
		if source == "" {
			source = "control"
		}
		log.Printf("keyboard request source=%s", source)
		c.logFocus("keyboard-request")
		c.refreshFullscreen()
		if c.quiet || c.fullscreen || c.stacks[0].native == nil {
			return nil
		}
		if err := c.enterMode(c.stacks[0], "KeyboardActive"); err != nil {
			return err
		}
		if m.Window == c.control {
			c.control.Hide()
		}
	case "toggle-arrange":
		return c.setArranging(!c.arranging)
	case "arrange":
		return c.setArranging(m.Enabled)
	case "close-quick":
		c.hideQuick(true)
	case "quiet":
		c.quiet = !c.quiet
		c.exitModes(true)
		c.hideQuick(false)
		c.updateVisibility()
	case "side":
		if m.Side != "left" && m.Side != "right" {
			return nil
		}
		for _, stack := range c.stacks {
			stack.side = m.Side
		}
		return c.layout()
	case "offset":
		if m.Offset >= 0 && m.Offset <= 1 {
			for _, stack := range c.stacks {
				stack.offset = m.Offset
			}
			return c.layout()
		}
	case "density":
		if m.ItemHeight >= 40 && m.ItemHeight <= 64 {
			for _, stack := range c.stacks {
				stack.itemHeight = m.ItemHeight
			}
			return c.layout()
		}
	case "displays":
		c.hideQuick(false)
		return c.layout()
	case "blur":
		if !platform.ForegroundIsOurs() && !(c.arranging && platform.ForegroundApplicationIsOurs() && c.control.IsFocused()) {
			c.exitModes(false)
			c.hideQuick(false)
		}
	case "foreground":
		if platform.ForegroundApplicationIsOurs() && c.store != nil {
			c.post(messageForReminders())
			c.post(message{Type: "settings-refresh"})
		}
		c.logFocus("foreground-notification")
		if !platform.ForegroundIsOurs() && !(c.arranging && platform.ForegroundApplicationIsOurs() && c.control.IsFocused()) {
			c.exitModes(false)
			c.hideQuick(false)
			c.fullscreen = platform.IsFullscreen()
			c.updateVisibility()
			c.scheduleFullscreenCheck()
		}
	case "fullscreen-check":
		// Rechecking a settled external window must not close a passive card
		// when the fullscreen state is unchanged.
		c.refreshFullscreen()
	case "open-notification-settings", "open-login-settings":
		return platform.OpenSystemSettings(m.Type == "open-login-settings")
	case "show-settings":
		c.cancelStackDrag()
		c.control.Show()
		c.control.Focus()
		c.control.EmitEvent("settings:open", true)
		c.post(message{Type: "settings-refresh"})
	case "show-control":
		c.control.EmitEvent("settings:open", false)
		c.cancelStackDrag()
		c.control.Show()
		c.control.Focus()
	case "hide-control":
		c.control.Hide()
	case "quit":
		if c.reminderTimer != nil {
			c.reminderTimer.Stop()
		}
		if c.store != nil {
			platform.StopNotifications()
		}
		if c.watchCleanup != nil {
			c.watchCleanup()
		}
		for _, o := range append(append([]*overlay{}, c.stacks...), c.quick) {
			if o.native != nil {
				o.native.Close()
			}
		}
		if c.quickTimer != nil {
			c.quickTimer.Stop()
		}
		if c.fullscreenTimer != nil {
			c.fullscreenTimer.Stop()
		}
		if c.cleanupTimer != nil {
			c.cleanupTimer.Stop()
		}
		c.app.Quit()
	}
	return nil
}
func (c *controller) layout() error {
	c.cancelStackDrag()
	displays, err := platform.Displays()
	if err != nil {
		return err
	}
	if len(displays) == 0 {
		return nil
	}
	primary := displays[0]
	for _, display := range displays {
		if display.Primary {
			primary = display
		}
	}
	used := map[string]bool{}
	for _, o := range c.stacks {
		if o.native == nil {
			continue
		}
		display := primary
		if o.displayID == "" && o.index > 0 {
			for _, candidate := range displays {
				if candidate.ID != primary.ID {
					display = candidate
					break
				}
			}
		}
		for _, candidate := range displays {
			if candidate.ID == o.displayID {
				display = candidate
			}
		}
		key := display.ID + o.side
		if used[key] {
			if o.side == "right" {
				o.side = "left"
			} else {
				o.side = "right"
			}
		}
		used[display.ID+o.side] = true
		o.displayID = display.ID
		if c.store != nil {
			for _, saved := range c.snapshot.Stacks {
				if saved.ID == o.stackID && (saved.DisplayID != o.displayID || saved.Side != o.side) {
					saved.DisplayID = o.displayID
					saved.Side = o.side
					c.post(message{Type: "persist-stack", Stack: saved})
				}
			}
		}
		area := display.WorkArea
		width := 312 * display.Scale
		if width > area.Width {
			width = area.Width
		}
		x := area.X
		if o.side == "right" {
			x = area.X + area.Width - width
		}
		if err := o.native.Move(platform.Rect{X: x, Y: area.Y, Width: width, Height: area.Height}); err != nil {
			return err
		}
		o.window.EmitEvent("spike:config", c.stackConfig(o))
		o.window.EmitEvent("spike:measure")
		log.Printf("layout platform=%s window=%s display=%s side=%s offset=%.2f coordinateScale=%.2f backingScale=%.2f workArea=%+v", runtime.GOOS, o.window.Name(), display.ID, o.side, o.offset, display.Scale, display.BackingScale, area)
	}
	if c.store != nil {
		c.control.EmitEvent("spike:config", c.stackConfig(c.stacks[0]))
	}
	c.fullscreen = platform.IsFullscreen()
	c.updateVisibility()
	c.scheduleFullscreenCheck()
	return nil
}
func (c *controller) refreshFullscreen() {
	next := platform.IsFullscreen()
	if next != c.fullscreen {
		c.fullscreen = next
		c.exitModes(false)
		c.hideQuick(false)
		c.updateVisibility()
		log.Printf("fullscreen=%t after interaction / Space settling", next)
	}
	c.scheduleFullscreenCheck()
}
func (c *controller) scheduleFullscreenCheck() {
	if c.fullscreenTimer != nil {
		c.fullscreenTimer.Stop()
		c.fullscreenTimer = nil
	}
	// macOS exposes no ordinary workspace notification for a borderless
	// fullscreen window resizing in the same app/Space. Only an already known
	// fullscreen session uses a slow fallback; normal idle has no such timer.
	if c.fullscreen && runtime.GOOS == "darwin" {
		c.fullscreenTimer = time.AfterFunc(time.Second, func() { c.post(message{Type: "fullscreen-check"}) })
	}
}
func (c *controller) updateVisibility() {
	for _, o := range c.stacks {
		if o.native != nil {
			if c.quiet || c.fullscreen {
				o.native.Hide()
			} else {
				o.native.ShowInactive()
			}
		}
	}
}
func (c *controller) enterMode(o *overlay, mode string) error {
	if c.quiet || c.fullscreen || o.native == nil {
		return nil
	}
	if mode != "KeyboardActive" && mode != "Editing" {
		mode = "KeyboardActive"
	}
	active := false
	if mode == "Editing" && o.mode != "Editing" {
		o.editReturnMode = "Passive"
	}
	for _, candidate := range append(append([]*overlay{}, c.stacks...), c.quick) {
		if candidate.mode != "Passive" {
			active = true
		}
		if mode == "Editing" && candidate.mode == "KeyboardActive" {
			o.editReturnMode = "KeyboardActive"
		}
	}
	if !active {
		c.previous = platform.CaptureForeground()
		if c.traceFocus {
			log.Printf("focus captured token=%s", c.previous.Diagnostic())
		}
	}
	c.exitModesExcept(false, o)
	if err := o.native.Activate(); err != nil {
		return err
	}
	o.window.Focus()
	o.mode = mode
	c.logFocus("mode-" + mode)
	logNativeWindow(o)
	o.window.EmitEvent("spike:mode", mode)
	c.control.EmitEvent("spike:mode", mode)
	c.scheduleQuickClose()
	log.Printf("input window=%s mode=%s", o.window.Name(), mode)
	return nil
}
func (c *controller) exitModes(restore bool) {
	c.exitModesExcept(restore, nil)
}
func (c *controller) exitModesExcept(restore bool, keep *overlay) {
	if keep == nil {
		c.cancelStackDrag()
	}
	if keep == nil && c.arranging {
		c.arranging = false
		c.emitPresentation()
	}
	wasActive := false
	for _, o := range append(append([]*overlay{}, c.stacks...), c.quick) {
		if o == keep {
			continue
		}
		if o.native != nil && o.mode != "Passive" {
			wasActive = true
			o.mode = "Passive"
			if err := o.native.Passive(); err != nil {
				log.Print(err)
			}
			o.window.EmitEvent("spike:mode", "Passive")
			logNativeWindow(o)
		}
	}
	if restore && wasActive {
		restored := c.previous.Restore()
		if c.traceFocus {
			log.Printf("focus restore requested success=%t token=%s", restored, c.previous.Diagnostic())
		}
		if !restored {
			log.Print("previous foreground window could not be restored")
		}
	}
	if wasActive {
		c.logFocus("exit-modes")
		c.control.EmitEvent("spike:mode", "Passive")
	}
}
func (c *controller) logFocus(boundary string) {
	if c.traceFocus {
		log.Printf("focus boundary=%s %s", boundary, platform.FocusDiagnostic())
	}
}
func (c *controller) hideQuick(restore bool) {
	c.quickSession.Close()
	c.pointers["quick"] = false
	c.scheduleQuickClose()
	c.emitPresentation()
	if c.quick.native != nil {
		c.exitModes(restore)
		c.quick.native.Hide()
	}
}

func (c *controller) stackConfig(o *overlay) map[string]any {
	return map[string]any{"side": o.side, "offset": o.offset, "stackIndex": o.index, "stackCount": len(c.stacks), "stackId": o.stackID, "itemHeight": o.itemHeight}
}
func (c *controller) emitPresentation() {
	sourceIndex := -1
	for _, o := range c.stacks {
		if o.window.Name() == c.quickSession.Source {
			sourceIndex = o.index
		}
	}
	c.app.Event.Emit("spike:presentation", map[string]any{"quickOpen": c.quickSession.Open, "sourceIndex": sourceIndex, "todoId": c.quickSession.TodoID, "quiet": c.quiet, "arranging": c.arranging})
}

func (c *controller) setArranging(enabled bool) error {
	if !enabled {
		c.exitModes(true)
		return nil
	}
	c.refreshFullscreen()
	if c.store == nil || c.quiet || c.fullscreen {
		return nil
	}
	c.hideQuick(false)
	c.arranging = true
	if err := c.enterMode(c.stacks[0], "KeyboardActive"); err != nil {
		c.arranging = false
		c.emitPresentation()
		return err
	}
	c.control.Hide()
	c.emitPresentation()
	return nil
}
func (c *controller) scheduleQuickClose() {
	if c.quickTimer != nil {
		c.quickTimer.Stop()
		c.quickTimer = nil
	}
	if !c.quickSession.Open || c.interactionActive() || c.quickSession.CloseAt.IsZero() {
		return
	}
	revision := c.quickSession.Revision
	c.quickTimer = time.AfterFunc(time.Until(c.quickSession.CloseAt), func() {
		c.post(message{Type: "quick-expire", Revision: revision})
	})
}
func (c *controller) interactionActive() bool {
	if c.quick.mode != "Passive" {
		return true
	}
	for _, o := range c.stacks {
		if o.mode != "Passive" {
			return true
		}
	}
	return false
}
func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 3; y < 29; y++ {
		for x := 3; x < 29; x++ {
			img.Set(x, y, color.NRGBA{R: 48, G: 57, B: 52, A: 255})
		}
	}
	for x := 8; x < 24; x++ {
		y := 22 - (x - 12)
		if x < 13 {
			y = 16 + (x - 8)
		}
		for d := 0; d < 3; d++ {
			img.Set(x, y+d, color.NRGBA{R: 247, G: 246, B: 243, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}
