package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	output := flag.String("output", "", "optional diagnostic directory; SIGUSR1 captures owned content")
	logPath := flag.String("log-file", "", "optional experiment log file")
	startCard := flag.Bool("card", false, "start with the first synthetic task's card visible")
	quitAfter := flag.Duration("quit-after", 0, "optional automatic quit, e.g. 2m")
	probe := flag.Bool("probe", false, "show an owned click-through probe behind the sidebar")
	probeOnly := flag.Bool("probe-only", false, "show only the owned background/input probe, for a separate fixture process")
	nativeCheck := flag.Bool("native-check", false, "check owned AppKit windows and lifecycle; requires -probe and -output")
	traceInput := flag.Bool("trace-input", false, "log delivered mouse events of this lab only; no global monitoring")
	directInput := flag.Bool("direct-input-fixture", false, "CUA-only render-window input fixture; requires -trace-input and -output; disables overlay hit testing")
	hybrid := flag.Bool("hybrid", false, "use a Web task window alongside the native sidebar; synthetic in-memory tasks only")
	nativeMain := flag.Bool("native-main", false, "use a Go-native task window alongside the native sidebar; no WebView, synthetic tasks only")
	mainHidden := flag.Bool("main-hidden", false, "start without a task main window; reopen using File > 打开任务窗口")
	flag.Parse()
	if *nativeCheck && (!*probe || *probeOnly || *output == "") {
		log.Fatal("-native-check requires -probe and -output, without -probe-only")
	}
	if *directInput && (!*traceInput || *output == "" || *nativeCheck || *probeOnly) {
		log.Fatal("-direct-input-fixture requires -trace-input and -output, without -native-check or -probe-only")
	}
	if *mainHidden && !(*hybrid || *nativeMain) {
		log.Fatal("-main-hidden requires -hybrid or -native-main")
	}
	if *hybrid && *nativeMain {
		log.Fatal("choose -hybrid or -native-main")
	}
	if (*hybrid || *nativeMain) && (*nativeCheck || *probeOnly) {
		log.Fatal("task main modes cannot be combined with -native-check or -probe-only")
	}
	checkFailed := false
	if *logPath != "" {
		if err := os.MkdirAll(filepath.Dir(*logPath), 0755); err != nil {
			log.Fatal(err)
		}
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.SetOutput(file)
	}
	if *output != "" {
		abs, err := filepath.Abs(*output)
		if err != nil {
			log.Fatal(err)
		}
		*output = abs
		if err := os.MkdirAll(*output, 0755); err != nil {
			log.Fatal(err)
		}
		mygo.App.SetPath(mygo.PathUserData, filepath.Join(*output, "app-data"))
	}
	mygo.App.SetName("Sidelet MyGo Lab")
	m := newModel()
	v := &views{m: m}
	input := newOverlayInput(m)
	input.directFixture = *directInput
	var h *hybridApp
	if *hybrid {
		h = newHybridApp(m)
	} else if *nativeMain {
		h = newNativeApp(m)
	}
	if h != nil {
		v.service = h.service
		if h.native {
			v.enableCloseTimer()
			v.renderPresence = input.directFixture
		}
		input.fallbackWindow = func() *mygo.Window { return h.window }
	}
	var stack, card, backgroundProbe *mygo.Window
	cardAnchorY := 0
	resizeCard := func() {
		if card == nil || m.UITheme == "" {
			return
		}
		bounds := card.Bounds()
		height := cardHeight
		if m.Editing {
			height = 390
		} else if m.CardReadHeight > 0 {
			height = m.CardReadHeight
		}
		if bounds.Height != height {
			bounds.Height = height
			bounds.Y = max(m.WorkY+10, min(m.WorkY+m.WorkHeight-height-10, cardAnchorY))
			card.SetBounds(bounds)
		}
	}
	logState := func(event string) {
		data := map[string]any{"at": time.Now().UTC(), "event": event, "pid": os.Getpid(), "model": m, "native": nativeState(), "input": input.state()}
		if stack != nil {
			data["stackBounds"], data["stackFocused"] = stack.Bounds(), stack.IsFocused()
		}
		if card != nil {
			data["cardBounds"], data["cardFocused"], data["cardVisible"] = card.Bounds(), card.IsFocused(), card.IsVisible()
		}
		if backgroundProbe != nil {
			data["probeBounds"], data["probeFocused"] = backgroundProbe.Bounds(), backgroundProbe.IsFocused()
		}
		if h != nil {
			data["taskSnapshot"] = h.service.snapshot()
			data["notificationAuthorization"], data["notificationStatus"] = h.service.notificationAuthorization, h.service.notificationStatus
			data["notificationDelivered"] = h.service.notificationDelivered
			data["mainRenderer"] = "web"
			if h.native {
				data["mainRenderer"] = "native"
			}
			data["mainOpen"] = h.window != nil
			if h.window != nil {
				data["mainID"], data["mainFocused"] = h.window.ID(), h.window.IsFocused()
			}
		}
		b, err := json.Marshal(data)
		if err != nil {
			log.Print(err)
			return
		}
		log.Print(string(b))
	}
	input.onEvent = logState
	input.pointerPresence = v.presence
	v.changed = func(event string) {
		resizeCard()
		if event == "cancel" || event == "save" || event == "close" {
			input.endEdit()
		}
		if stack != nil {
			if !m.cardOpen() && card != nil {
				card.Hide()
				input.endEdit()
			}
			if m.UITheme != "" && stack.Bounds().Height != m.stackHeight() && !m.Dragging {
				m.positionStack()
			}
			m.Y = max(m.WorkY, min(m.WorkY+max(0, m.WorkHeight-m.stackHeight()), m.Y))
			stack.SetBounds(mygo.Rectangle{X: m.X, Y: m.Y, Width: stackWidth, Height: m.stackHeight()})
			stack.Invalidate()
			if card != nil {
				card.Invalidate()
			}
		}
		input.sync()
		if h != nil {
			h.nativeChanged(event)
			if h.editItem != nil {
				h.editItem.SetEnabled(m.Opened >= 0 && !m.Editing)
			}
		}
		logState(event)
	}
	if h != nil {
		h.onState = logState
		h.service.changed = func(event string) { v.changed(event); h.publish() }
	}
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { input.close() })
	mygo.App.WhenReady(func() {
		if *traceInput {
			stopTrace := startInputTrace()
			mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { stopTrace() })
		}
		if *quitAfter > 0 {
			time.AfterFunc(*quitAfter, func() { mygo.App.Quit() })
		}
		d := mygo.Screen.PrimaryDisplay().WorkArea
		m.WorkX, m.WorkY, m.WorkWidth, m.WorkHeight = d.X, d.Y, d.Width, d.Height
		m.X, m.Y = d.X, d.Y+int(float64(d.Height-stackHeight)*0.45)
		if m.UITheme != "" {
			m.positionStack()
		}
		if *probe || *probeOnly {
			clicks := 0
			probeText, lastProbeText := "", ""
			p := mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet MyGo 穿透验证", X: m.X, Y: m.Y, Width: 520, Height: 300, Content: ui.View(func(c *ui.Context) {
				ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
					ui.Text(c, "这是实验自己的背景窗口").Bold()
					if ui.Button(c, fmt.Sprintf("空白穿透计数：%d", clicks)).Height(90).Clicked() {
						clicks++
						log.Printf("probe-click count=%d", clicks)
					}
					ui.Text(c, "只点击侧栏没有绘制内容的位置。")
					ui.TextInput(c, &probeText).Label("后台输入验证").Height(32)
					if probeText != lastProbeText {
						lastProbeText = probeText
						log.Printf("probe-text %q", probeText)
					}
				})
			})})
			p.OnClosed(func() { log.Print("probe closed") })
			backgroundProbe = p
			if *probeOnly {
				focusEditor(p)
				logState("probe-ready")
				return
			}
		}
		stack = mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet MyGo 侧栏", X: m.X, Y: m.Y, Width: stackWidth, Height: m.stackHeight(),
			Frameless: true, Transparent: true, AlwaysOnTop: true, DisableResize: true, DisableShadow: true, Hidden: true, Content: ui.View(v.stack)})
		input.bindStack(stack)
		input.moveDrag = func(p mygo.Point) {
			m.moveDrag(p.X, p.Y)
			stack.SetBounds(mygo.Rectangle{X: m.X, Y: m.Y, Width: stackWidth, Height: m.stackHeight()})
			input.sync()
		}
		v.pointer = func(index int, ev ui.InputEvent) bool {
			// Use the delivered event, not a later sample of the system cursor.
			// HandleInput coordinates are relative to the marker's box.
			p := eventScreenPoint(m, index, ev)
			switch ev.Kind {
			case ui.InputPointerDown:
				if ev.Button != 0 {
					return false
				}
				if m.UITheme != "" {
					if m.Editing {
						return true
					}
					if m.cardOpen() {
						v.closeCard()
					}
				}
				m.startDrag(p.X, p.Y)
				input.sync()
				logState("drag-start")
				return true
			case ui.InputPointerMove:
				if !m.Dragging {
					return false
				}
				if input.handlesNativeDrag() {
					return true
				}
				m.moveDrag(p.X, p.Y)
				stack.SetBounds(mygo.Rectangle{X: m.X, Y: m.Y, Width: stackWidth, Height: m.stackHeight()})
				input.sync()
				return true
			case ui.InputPointerUp:
				if !m.Dragging {
					return false
				}
				m.endDrag()
				v.armClose()
				stack.SetBounds(mygo.Rectangle{X: m.X, Y: m.Y, Width: stackWidth, Height: m.stackHeight()})
				input.sync()
				logState("drag-end")
				return true
			}
			return false
		}
		stack.OnBlur(func() {
			v.order.Epoch++
			if m.Dragging {
				m.cancelDrag()
				stack.SetPosition(m.X, m.Y)
				stack.Invalidate()
				logState("drag-cancel-blur")
			}
		})
		stack.OnResize(func() { v.order.Epoch++ })
		v.showAll = func() {
			if h != nil {
				h.show()
				if h.view != nil {
					h.view.settingsOpen = false
					h.window.Invalidate()
				}
			}
		}
		v.open = func(i int) {
			if m.UITheme != "" {
				v.beginSession(i)
			}
			if card == nil {
				card = mygo.NewWindow(mygo.WindowOptions{Title: "Sidelet MyGo 任务卡片", Width: cardWidth, Height: cardHeight, Hidden: true, Frameless: true,
					Transparent: true, AlwaysOnTop: true, DisableResize: true, Content: ui.View(v.card)})
				input.bindCard(card)
			}
			x := m.X + 20
			if m.Side == "right" {
				x = m.X - 28
			}
			x = max(d.X, min(d.X+d.Width-cardWidth, x))
			y := max(d.Y, min(d.Y+d.Height-cardHeight, m.Y+8+m.rowFor(i)*rowPitch))
			if m.UITheme != "" {
				x, y = m.cardAnchor(i)
			}
			cardAnchorY = y
			if m.UITheme != "" {
				y = max(m.WorkY+10, min(m.WorkY+m.WorkHeight-cardHeight-10, y))
			}
			card.SetBounds(mygo.Rectangle{X: x, Y: y, Width: cardWidth, Height: cardHeight})
			card.Invalidate()
			card.ShowInactive()
			input.sync()
			if h != nil && h.editItem != nil {
				h.editItem.SetEnabled(true)
			}
			logState("open")
		}
		v.edit = func() {
			v.editorFocusPending = true
			resizeCard()
			input.beginEdit(card)
		}
		v.close = func() {
			if card != nil {
				card.Hide()
			}
			stack.Invalidate()
			input.sync()
		}
		stack.ShowInactive()
		input.sync()
		logState("ready")
		if h != nil {
			h.service.open = v.open
			h.editCard = func() {
				if m.Opened >= 0 && !m.Editing {
					m.edit()
					v.edit()
					v.notify("edit")
				}
			}
			h.installMenu()
			mygo.App.OnActivate(func(bool) {
				if h.window == nil && !m.Editing {
					h.show()
				}
			})
			if !*mainHidden {
				h.show()
			}
		}
		if *startCard {
			m.open(0)
			v.open(0)
		}
		if *nativeCheck {
			runNativeChecks(input, v, backgroundProbe, *output, func(ok bool) { checkFailed = !ok })
		}
		if *output != "" {
			go watchCaptures(*output, func() {
				stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
				var targets map[string]*mygo.Window
				var stateJSON []byte
				mygo.RunOnMain(func() {
					targets = map[string]*mygo.Window{"stack": stack, "card": card}
					if h != nil {
						targets["main"] = h.window
					}
					logState("capture")
					var mem runtime.MemStats
					runtime.ReadMemStats(&mem)
					stateJSON, _ = json.MarshalIndent(map[string]any{"model": m, "goHeapAlloc": mem.HeapAlloc, "goSys": mem.Sys, "native": nativeState()}, "", "  ")
				})
				for name, w := range targets {
					if w != nil && w.IsVisible() {
						b, err := w.CapturePage()
						if err == nil {
							err = os.WriteFile(filepath.Join(*output, stamp+"-"+name+".png"), b, 0644)
						}
						if err != nil {
							log.Print(err)
						}
					}
				}
				if err := os.WriteFile(filepath.Join(*output, stamp+"-state.json"), stateJSON, 0644); err != nil {
					log.Print(err)
				}
			})
		}
	})
	// SIGINT only stops this lab, never an installed Sidelet instance.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() { <-stop; mygo.App.Quit() }()
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	if checkFailed {
		os.Exit(1)
	}
}
