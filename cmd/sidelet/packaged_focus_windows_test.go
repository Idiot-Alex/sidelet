//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"
)

// All system input injection is test-only and restricted to disposable hosted
// CI. The companion uses the ZIP EXE's existing probe-only mode in a separate
// process. It has its own input field, UI thread and Windows input queue.
// No SendMessage, model mutations, foreground grants or AttachThreadInput are
// used to drive the app. Every keyboard batch checks the exact foreground HWND;
// mouse clicks check Windows hit testing against a window of an owned process.
type focusDriver struct {
	t                *testing.T
	owned            map[uint32]bool
	appLog, probeLog string
	stages           []string
}

type focusInput struct {
	Kind uint32
	_    uint32 // INPUT's union is 8-byte aligned on both Windows targets.
	Data [32]byte
}

type focusLog struct {
	Event                     string                   `json:"event"`
	PID                       int                      `json:"pid"`
	Model                     model                    `json:"model"`
	CardVisible               bool                     `json:"cardVisible"`
	QuickVisible              bool                     `json:"quickAddVisible"`
	QuickRegistered           bool                     `json:"quickAddShortcutRegistered"`
	QuickText                 string                   `json:"quickAddText"`
	StorageError              string                   `json:"storageError"`
	MainOpen                  bool                     `json:"mainOpen"`
	NotificationSession       notificationSessionState `json:"notificationSession"`
	NotificationClicks        int                      `json:"notificationClicks"`
	NotificationFailed        bool                     `json:"notificationFailed"`
	NotificationAuthorization string                   `json:"notificationAuthorization"`
}

func focusPID(hwnd uintptr) uint32 {
	var pid uint32
	quickGetPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func focusWindow(pid int, title string) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if focusPID(hwnd) != uint32(pid) {
			return 1
		}
		var name [256]uint16
		inputUser32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if syscall.UTF16ToString(name[:]) == title {
			found = hwnd
			return 0
		}
		return 1
	})
	inputUser32.NewProc("EnumWindows").Call(cb, 0)
	return found
}

func (d *focusDriver) wait(label string, check func() bool) {
	d.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	front, _, _ := quickGetForeground.Call()
	d.t.Fatalf("%s: timed out; foreground=%#x pid=%d lastApp=%+v", label, front, focusPID(front), d.app())
}

func (d *focusDriver) app() focusLog {
	data, _ := os.ReadFile(d.appLog)
	var last focusLog
	for _, line := range strings.Split(string(data), "\n") {
		at := strings.IndexByte(line, '{')
		if at < 0 {
			continue
		}
		var entry focusLog
		if json.Unmarshal([]byte(line[at:]), &entry) == nil {
			last = entry
		}
	}
	return last
}

func (d *focusDriver) probeText() string {
	data, _ := os.ReadFile(d.probeLog)
	last := ""
	for _, line := range strings.Split(string(data), "\n") {
		const prefix = "probe-text "
		if at := strings.Index(line, prefix); at >= 0 {
			if value, err := strconv.Unquote(line[at+len(prefix):]); err == nil {
				last = value
			}
		}
	}
	return last
}

func (d *focusDriver) foreground(hwnd uintptr) {
	d.t.Helper()
	d.wait("foreground returned to expected window", func() bool {
		front, _, _ := quickGetForeground.Call()
		return front == hwnd && d.owned[focusPID(hwnd)]
	})
}

func (d *focusDriver) send(inputs ...focusInput) {
	d.t.Helper()
	if unsafe.Sizeof(focusInput{}) != 40 {
		d.t.Fatal("incorrect Win64 INPUT layout")
	}
	n, _, err := inputUser32.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if int(n) != len(inputs) {
		d.t.Fatalf("SendInput inserted %d/%d: %v", n, len(inputs), err)
	}
}

func focusKey(vk, scan uint16, flags uint32) focusInput {
	in := focusInput{Kind: 1}
	binary.LittleEndian.PutUint16(in.Data[0:2], vk)
	binary.LittleEndian.PutUint16(in.Data[2:4], scan)
	binary.LittleEndian.PutUint32(in.Data[4:8], flags)
	return in
}

func (d *focusDriver) keys(hwnd uintptr, keys ...uint16) {
	d.t.Helper()
	d.foreground(hwnd)
	var inputs []focusInput
	for _, key := range keys {
		inputs = append(inputs, focusKey(key, 0, 0))
	}
	for i := len(keys) - 1; i >= 0; i-- {
		inputs = append(inputs, focusKey(keys[i], 0, 2))
	}
	d.send(inputs...)
}

func (d *focusDriver) text(hwnd uintptr, value string) {
	d.t.Helper()
	d.foreground(hwnd)
	var inputs []focusInput
	for _, unit := range utf16.Encode([]rune(value)) {
		inputs = append(inputs, focusKey(0, unit, 4), focusKey(0, unit, 6))
	}
	d.send(inputs...)
}

// Coordinates are client DIPs. The probe's input field and the card's equal
// width footer buttons are stable UI targets; native hit testing guards them.
func (d *focusDriver) point(hwnd uintptr, x, y int) winPoint {
	d.t.Helper()
	if !d.owned[focusPID(hwnd)] {
		d.t.Fatal("refusing an unowned window")
	}
	class, _ := syscall.UTF16PtrFromString("MyGoSurface")
	surface, _, _ := inputFindChild.Call(hwnd, 0, uintptr(unsafe.Pointer(class)), 0)
	if surface == 0 {
		d.t.Fatal("owned MyGo surface not found")
	}
	dpi, _, _ := inputGetDPI.Call(hwnd)
	if dpi == 0 {
		d.t.Fatal("window DPI unavailable")
	}
	p := winPoint{int32(winPX(x, int(dpi))), int32(winPX(y, int(dpi)))}
	if ok, _, err := inputClientScreen.Call(surface, uintptr(unsafe.Pointer(&p))); ok == 0 {
		d.t.Fatal(err)
	}
	return p
}

func (d *focusDriver) move(hwnd uintptr, x, y int) {
	d.t.Helper()
	p := d.point(hwnd, x, y)
	metrics := inputUser32.NewProc("GetSystemMetrics")
	get := func(code uintptr) int32 { n, _, _ := metrics.Call(code); return int32(n) }
	x0, y0, w, h := get(76), get(77), get(78), get(79)
	if w < 2 || h < 2 {
		d.t.Fatal("no virtual desktop")
	}
	in := focusInput{}
	binary.LittleEndian.PutUint32(in.Data[0:4], uint32(int64(p.X-x0)*65535/int64(w-1)))
	binary.LittleEndian.PutUint32(in.Data[4:8], uint32(int64(p.Y-y0)*65535/int64(h-1)))
	binary.LittleEndian.PutUint32(in.Data[12:16], 0xc001) // MOVE | ABSOLUTE | VIRTUALDESK
	d.send(in)
	d.wait("mouse reached owned window", func() bool {
		var actual winPoint
		ok, _, _ := inputGetCursor.Call(uintptr(unsafe.Pointer(&actual)))
		return ok != 0 && actual.X >= p.X-2 && actual.X <= p.X+2 && actual.Y >= p.Y-2 && actual.Y <= p.Y+2 && inputHit(actual) == hwnd
	})
	time.Sleep(100 * time.Millisecond)
}

func (d *focusDriver) click(hwnd uintptr, x, y int) {
	d.t.Helper()
	d.move(hwnd, x, y)
	var actual winPoint
	inputGetCursor.Call(uintptr(unsafe.Pointer(&actual)))
	if inputHit(actual) != hwnd {
		d.t.Fatal("refusing click on an occluded window")
	}
	down, up := focusInput{}, focusInput{}
	binary.LittleEndian.PutUint32(down.Data[12:16], 2)
	binary.LittleEndian.PutUint32(up.Data[12:16], 4)
	d.send(down, up)
}

func (d *focusDriver) stage(name string) {
	d.t.Helper()
	if err := d.app().StorageError; err != "" {
		d.t.Fatal(err)
	}
	d.stages = append(d.stages, name)
	d.t.Log("passed:", name)
}

func TestPackagedWindowsCrossProcessFocus(t *testing.T) {
	executable := os.Getenv("SIDELET_SMOKE_EXE")
	if executable == "" {
		t.Skip("requires ZIP executable and disposable Windows input desktop")
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("system input test is restricted to disposable GitHub-hosted Windows")
	}
	root := t.TempDir()
	if output := os.Getenv("SIDELET_SMOKE_OUTPUT"); output != "" {
		if err := os.MkdirAll(output, 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		root, err = os.MkdirTemp(output, "cross-process-")
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("retained synthetic evidence:", root)
	d := &focusDriver{t: t, owned: map[uint32]bool{}, appLog: filepath.Join(root, "app.log"), probeLog: filepath.Join(root, "probe.log")}
	defer func() {
		data, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "stages": d.stages, "scope": "separate process input/caret and system SendInput", "hardwareInputVerified": false, "imeVerified": false}, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "cross-process.json"), data, 0600); err != nil {
			t.Error(err)
		}
	}()
	// Seed only this profile; real application operations below must persist.
	dir := filepath.Join(root, "profile")
	m := newModel()
	m.UITheme = "mac"
	p, err := openProfile(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	s := testService(m)
	if _, err = s.AddWithPin("CI original task", "", 0, true); err != nil {
		p.close()
		t.Fatal(err)
	}
	if err = m.commit(func() error { m.Side = "left"; m.Offset = .35; return nil }); err != nil {
		p.close()
		t.Fatal(err)
	}
	p.close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	start := func(name, logPath string, args ...string) *exec.Cmd {
		args = append(args, "-output", filepath.Join(root, name), "-log-file", logPath, "-quit-after", "70s")
		cmd := exec.CommandContext(ctx, executable, args...)
		out, err := os.Create(filepath.Join(root, name+"-process.log"))
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			out.Close()
			t.Fatal(err)
		}
		d.owned[uint32(cmd.Process.Pid)] = true
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = out.Close() })
		return cmd
	}
	app := start("app", d.appLog, "-data-dir", dir, "-main-hidden")
	d.wait("app startup and production shortcut", func() bool { return d.app().QuickRegistered })
	probe := start("probe", d.probeLog, "-native-main=false", "-probe-only")
	if app.Process.Pid == probe.Process.Pid {
		t.Fatal("companion is not independent")
	}
	var probeWindow, stack uintptr
	d.wait("companion window", func() bool {
		probeWindow = focusWindow(probe.Process.Pid, "Sidelet MyGo 穿透验证")
		return probeWindow != 0
	})
	d.wait("sidebar window", func() bool { stack = focusWindow(app.Process.Pid, "Sidelet MyGo 侧栏"); return stack != 0 })
	var original winPoint
	inputGetCursor.Call(uintptr(unsafe.Pointer(&original)))
	t.Cleanup(func() {
		// Pair any modifiers/button left down by a failed batch, only while a
		// fixture is foreground. The companion/app are still alive at cleanup.
		front, _, _ := quickGetForeground.Call()
		if d.owned[focusPID(front)] {
			up := focusInput{}
			binary.LittleEndian.PutUint32(up.Data[12:16], 4)
			items := []focusInput{up, focusKey(0x10, 0, 2), focusKey(0x11, 0, 2), focusKey(0x12, 0, 2)}
			inputUser32.NewProc("SendInput").Call(uintptr(len(items)), uintptr(unsafe.Pointer(&items[0])), unsafe.Sizeof(items[0]))
		}
		inputUser32.NewProc("SetCursorPos").Call(uintptr(original.X), uintptr(original.Y))
	})
	d.click(probeWindow, 180, 214)
	d.text(probeWindow, "AB")
	d.wait("baseline text", func() bool { return d.probeText() == "AB" })
	d.keys(probeWindow, 0x25) // leave the caret between A and B
	prefix := "A"
	continueTyping := func(name, letter string) {
		d.text(probeWindow, letter)
		prefix += letter
		d.wait(name+" text/caret", func() bool { return d.probeText() == prefix+"B" })
		d.stage(name)
	}
	state := d.app().Model
	marker := state.markerRect(0)
	d.move(stack, marker.X+7, marker.Y+marker.Height/2)
	d.wait("sidebar expanded", func() bool { return d.app().Model.Hover == 0 })
	continueTyping("hover-keeps-foreign-input", "H")
	d.click(stack, 180, marker.Y+marker.Height/2)
	var card uintptr
	d.wait("read card", func() bool {
		card = focusWindow(app.Process.Pid, "Sidelet MyGo 任务卡片")
		return card != 0 && d.app().CardVisible
	})
	continueTyping("preview-click-keeps-foreign-input", "P")
	d.move(card, 100, 70)
	time.Sleep(650 * time.Millisecond) // exceed the card handoff grace period
	if !d.app().CardVisible {
		t.Fatal("card vanished during handoff")
	}
	continueTyping("card-handoff-keeps-foreign-input", "C")
	footer := func(index, count int) {
		dpi, _, _ := inputGetDPI.Call(card)
		var r winRect
		inputGetRect.Call(card, uintptr(unsafe.Pointer(&r)))
		width, height := winDIP(r.Right-r.Left, int(dpi)), winDIP(r.Bottom-r.Top, int(dpi))
		d.click(card, 16+(width-32)*(2*index+1)/(2*count), height-30)
	}
	edit := func(title string) {
		footer(2, 3)
		d.wait("card editor", func() bool { return d.app().Model.Editing })
		time.Sleep(150 * time.Millisecond) // commit the newly focused input layout
		d.keys(card, 0x11, 'A')
		d.text(card, title)
	}
	edit("CI discarded edit")
	footer(1, 2)
	d.wait("cancel card edit", func() bool { return !d.app().Model.Editing })
	if d.app().Model.Tasks[0].Title != "CI original task" {
		t.Fatal("cancel saved draft")
	}
	d.move(card, 100, 70) // stay inside the card after its editor shrinks
	continueTyping("card-cancel-restores-foreign-caret", "X")
	edit("CI saved edit")
	d.keys(card, 0x0d)
	d.wait("save card edit", func() bool { st := d.app(); return !st.Model.Editing && st.Model.Tasks[0].Title == "CI saved edit" })
	continueTyping("card-save-restores-foreign-caret", "S")
	// The real registered Ctrl+Shift+Space shortcut must open the actual panel.
	openQuick := func() uintptr {
		d.keys(probeWindow, 0x11, 0x10, 0x20)
		var quick uintptr
		d.wait("global shortcut opens quick add", func() bool {
			quick = focusWindow(app.Process.Pid, "Sidelet · 演示 · 快速添加")
			return quick != 0 && d.app().QuickVisible
		})
		d.foreground(quick)
		time.Sleep(150 * time.Millisecond)
		return quick
	}
	quick := openQuick()
	d.text(quick, "CI discarded quick task")
	d.keys(quick, 0x1b)
	d.wait("quick add cancelled", func() bool { return !d.app().QuickVisible })
	continueTyping("quick-cancel-restores-foreign-caret", "Q")
	quick = openQuick()
	d.text(quick, "CI saved quick task")
	d.keys(quick, 0x11, 0x0d) // Ctrl+Enter adds and closes; Enter continues.
	d.wait("quick task saved", func() bool {
		st := d.app()
		return !st.QuickVisible && len(st.Model.Tasks) == 2 && st.Model.Tasks[1].Title == "CI saved quick task"
	})
	continueTyping("quick-save-restores-foreign-caret", "N")
	// Clicking away is the user's new focus choice; the panel must not steal it.
	quick = openQuick()
	d.text(quick, "CI retained quick draft")
	d.click(probeWindow, 180, 214)
	d.wait("quick add lost focus", func() bool { return !d.app().QuickVisible })
	d.keys(probeWindow, 0x23) // explicit click moved the caret; now append at End
	d.text(probeWindow, "Z")
	d.wait("click-away keeps input", func() bool { return d.probeText() == prefix+"BZ" })
	d.stage("quick-click-away-respects-user-focus")
	quick = openQuick()
	if d.app().QuickText != "CI retained quick draft" {
		t.Fatal("blur lost draft")
	}
	d.keys(quick, 0x1b)
	d.wait("reopened draft cancelled", func() bool { return !d.app().QuickVisible })
	d.text(probeWindow, "R")
	d.wait("repeat quick add restores input", func() bool { return d.probeText() == prefix+"BZR" })
	d.stage("quick-reopen-preserves-draft-and-caret")
	// SQLite transactions can be read while the app runs, including WAL content.
	saved := readSmokeProfile(t, dir)
	if len(saved.Tasks) != 2 || saved.Tasks[0].Title != "CI saved edit" || saved.Tasks[1].Title != "CI saved quick task" {
		t.Fatal("actual input did not persist expected tasks")
	}
	d.stage("sqlite-contains-only-confirmed-edits")
	t.Logf("independent PIDs app=%d companion=%d; verified %d stages", app.Process.Pid, probe.Process.Pid, len(d.stages))
}
