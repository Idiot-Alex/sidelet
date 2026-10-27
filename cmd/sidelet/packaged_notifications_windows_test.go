//go:build windows

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"unsafe"
)

// The ZIP executable uses its real backend and SQLite. All mutations while it
// runs are ordinary card clicks through SendInput; there is no test RPC or fake
// delivery backend in the packaged program. Only disposable hosted CI may run.
func TestPackagedWindowsNotifications(t *testing.T) {
	executable := os.Getenv("SIDELET_SMOKE_EXE")
	if executable == "" {
		t.Skip("requires packaged EXE and disposable Windows desktop")
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("notification/system input checks require a disposable GitHub-hosted runner")
	}
	root := t.TempDir()
	if output := os.Getenv("SIDELET_SMOKE_OUTPUT"); output != "" {
		var err error
		root, err = os.MkdirTemp(output, "notifications-")
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("retained synthetic notification evidence:", root)
	stages := []string{}
	banner := map[string]any{"found": false, "invoked": false, "reason": "not probed"}
	defer func() {
		data, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "stages": stages, "banner": banner,
			"scope": "packaged EXE, real Shell submission, card input and SQLite receipts", "soundVerified": false, "notificationSettingsChanged": false}, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "notifications.json"), data, 0600); err != nil {
			t.Error(err)
		}
	}()
	stage := func(name string) { stages = append(stages, name); t.Log("passed:", name) }
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	title := fmt.Sprintf("Sidelet CI reminder %x", token)
	dir := filepath.Join(root, "profile 中文")
	m := newModel()
	m.UITheme = "mac"
	p, err := openProfile(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	s := testService(m)
	now := time.Now()
	for _, row := range []struct {
		title       string
		due         int64
		remind, pin bool
	}{
		{title, now.Add(-2 * time.Minute).UnixMilli(), true, true},
		{"CI future reminder", now.Add(time.Hour).UnixMilli(), true, false},
		{"CI reminder disabled", now.Add(-time.Minute).UnixMilli(), false, false},
		{"CI completed reminder", now.Add(-time.Minute).UnixMilli(), true, false},
		{"CI already acknowledged", now.Add(-time.Minute).UnixMilli(), true, false},
	} {
		if _, err := s.SaveTask(0, row.title, "独立合成提醒，不读取日常任务", 0, row.pin, row.due, row.remind, false, 0); err != nil {
			p.close()
			t.Fatal(err)
		}
	}
	if _, err := s.SetDone(4, true, m.Tasks[3].Version); err != nil {
		p.close()
		t.Fatal(err)
	}
	if err := m.commit(func() error {
		m.Tasks[4].ReminderSentAt = now.UnixMilli()
		m.Side, m.Offset = "left", .35
		return nil
	}); err != nil {
		p.close()
		t.Fatal(err)
	}
	p.close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	var cursor winPoint
	inputGetCursor.Call(uintptr(unsafe.Pointer(&cursor)))
	t.Cleanup(func() { inputUser32.NewProc("SetCursorPos").Call(uintptr(cursor.X), uintptr(cursor.Y)) })
	run := func(name string, duration time.Duration, interact func(*focusDriver, int)) focusLog {
		logPath := filepath.Join(root, name+".log")
		out, err := os.Create(filepath.Join(root, name+"-process.log"))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		cmd := exec.CommandContext(ctx, executable, "-data-dir", dir, "-main-hidden", "-log-file", logPath, "-quit-after", duration.String())
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		finished := false
		defer func() {
			if !finished {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}()
		d := &focusDriver{t: t, owned: map[uint32]bool{uint32(cmd.Process.Pid): true}, appLog: logPath}
		d.wait("notification app startup", func() bool { return d.app().QuickRegistered })
		interact(d, cmd.Process.Pid)
		err = cmd.Wait()
		finished = true
		if err != nil {
			t.Fatalf("notification process did not exit normally: %v", err)
		}
		last := d.app()
		if last.StorageError != "" || last.NotificationFailed || last.NotificationSession.Failed != 0 {
			t.Fatalf("real notification/SQLite error: %+v", last)
		}
		return last
	}
	openCard := func(d *focusDriver, pid int) uintptr {
		stack := focusWindow(pid, "Sidelet MyGo 侧栏")
		model := d.app().Model
		marker := model.markerRect(0)
		d.move(stack, marker.X+7, marker.Y+marker.Height/2)
		d.wait("notification task hover", func() bool { return d.app().Model.Hover == 0 })
		d.click(stack, 180, marker.Y+marker.Height/2)
		var card uintptr
		d.wait("notification task card", func() bool {
			card = focusWindow(pid, "Sidelet MyGo 任务卡片")
			return card != 0 && d.app().CardVisible
		})
		return card
	}
	footer := func(d *focusDriver, card uintptr, index, bottom int) {
		var r winRect
		inputGetRect.Call(card, uintptr(unsafe.Pointer(&r)))
		dpi, _, _ := inputGetDPI.Call(card)
		w, h := winDIP(r.Right-r.Left, int(dpi)), winDIP(r.Bottom-r.Top, int(dpi))
		d.click(card, 16+(w-32)*(2*index+1)/4, h-bottom)
	}
	first := run("deliver", 13*time.Second, func(d *focusDriver, _ int) {
		d.wait("overdue reminder submitted and acknowledged", func() bool {
			st := d.app()
			return len(st.Model.Tasks) == 5 && st.Model.Tasks[0].ReminderSentAt != 0 && st.NotificationSession.Submitted == 1
		})
		if d.app().MainOpen {
			t.Fatal("background reminder opened main window")
		}
		stage("hidden-main-delivers-overdue-reminder")
		saved := readSmokeProfile(t, dir)
		if saved.Tasks[0].ReminderSentAt == 0 || saved.Tasks[1].ReminderSentAt != 0 || saved.Tasks[2].ReminderSentAt != 0 || saved.Tasks[3].ReminderSentAt != 0 || saved.Tasks[4].ReminderSentAt != now.UnixMilli() {
			t.Fatal("submission consumed an ineligible task or failed to commit receipt")
		}
		stage("sqlite-receipt-and-ineligible-task-isolation")
		// A runner may suppress banners. Keep actual UI evidence separate from
		// the mandatory submission checks; never simulate a Shell click callback.
		probePath := filepath.Join(root, "banner.json")
		probeCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "probe-windows-notification.ps1"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(script); err != nil {
			t.Fatal(err)
		}
		// Use the same supported PowerShell host as the CI entry point. Legacy
		// Windows PowerShell can terminate before executing a script on runners.
		probe := exec.CommandContext(probeCtx, "pwsh", "-NoLogo", "-NoProfile", "-NonInteractive", "-STA", "-File", script, "-Title", title, "-OutputPath", probePath)
		output, probeErr := probe.CombinedOutput()
		stop()
		output = append(output, []byte(fmt.Sprintf("\nprobe exit: %v\n", probeErr))...)
		_ = os.WriteFile(filepath.Join(root, "banner-probe.log"), output, 0600)
		if data, err := os.ReadFile(probePath); err == nil {
			if err := json.Unmarshal(data, &banner); err != nil {
				t.Fatal(err)
			}
		} else {
			banner["reason"] = fmt.Sprintf("UI probe unavailable: %v", probeErr)
		}
		if banner["invoked"] == true {
			d.wait("actual notification activation", func() bool { st := d.app(); return st.NotificationClicks == 1 && st.MainOpen })
			stage("visible-banner-activation-opens-main")
		}
	})
	if first.NotificationSession.Submitted != 1 {
		t.Fatal("repeat reconciliation duplicated notification")
	}
	stage("reconciliation-does-not-resubmit")
	restarted := run("receipt-restart", 3*time.Second, func(d *focusDriver, _ int) {
		d.wait("acknowledged reminder reconciliation", func() bool { return d.app().NotificationAuthorization == "systemManaged" })
	})
	if restarted.NotificationSession.Submitted != 0 || restarted.Model.Tasks[0].ReminderSentAt != first.Model.Tasks[0].ReminderSentAt || restarted.Model.Tasks[0].Done {
		t.Fatal("restart changed or resent the real, uncompleted reminder receipt")
	}
	stage("restart-deduplicates-real-submission-receipt")

	// Reopen only after the actual process exits. Prepare a future deadline;
	// delivery must be driven by the production timer while the app stays hidden.
	m = newModel()
	m.UITheme = "mac"
	p, err = openProfile(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	s = testService(m)
	_, err = s.SetDone(1, false, m.Tasks[0].Version)
	if err == nil {
		_, err = s.SaveTask(1, title, "定时提醒合成任务", 0, true, time.Now().Add(3*time.Second).UnixMilli(), true, false, m.Tasks[0].Version)
	}
	if err != nil {
		p.close()
		t.Fatal(err)
	}
	p.close()
	second := run("timer-reminder", 8*time.Second, func(d *focusDriver, pid int) {
		d.wait("deadline timer delivery", func() bool {
			st := d.app()
			return len(st.Model.Tasks) == 5 && st.Model.Tasks[0].ReminderSentAt != 0 && st.NotificationSession.Submitted == 1
		})
		if d.app().MainOpen {
			t.Fatal("timer reminder opened main window")
		}
		stage("background-timer-delivers-due-reminder")

	})
	if second.NotificationSession.Submitted != 1 {
		t.Fatal("timer reminder repeated")
	}

	third := run("restart", 3*time.Second, func(d *focusDriver, _ int) {
		d.wait("restart reminder reconciliation", func() bool { return d.app().NotificationAuthorization == "systemManaged" })
	})
	if third.NotificationSession.Submitted != 0 {
		t.Fatal("restart resent acknowledged or future reminder")
	}
	stage("restart-does-not-resend-acknowledged-or-future-reminders")
	m = newModel()
	m.UITheme = "mac"
	p, err = openProfile(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	s = testService(m)
	_, err = s.Unsnooze(1, m.Tasks[0].Version)
	if err == nil {
		_, err = s.SaveTask(1, title, "完成撤回合成任务", 0, true, time.Now().Add(-time.Second).UnixMilli(), true, false, m.Tasks[0].Version)
	}
	p.close()
	if err != nil {
		t.Fatal(err)
	}
	completed := run("complete", 4*time.Second, func(d *focusDriver, pid int) {
		d.wait("completion fixture delivery", func() bool { return d.app().NotificationSession.Submitted == 1 })
		card := openCard(d, pid)
		footer(d, card, 0, 30)
		d.wait("complete withdraws accepted notification", func() bool {
			st := d.app()
			return st.Model.Tasks[0].Done && st.NotificationSession.Removed == 1 && len(st.NotificationSession.Accepted) == 0
		})
	})
	if completed.NotificationSession.Submitted != 1 || !readSmokeProfile(t, dir).Tasks[0].Done {
		t.Fatal("completion lost durable state or resent notification")
	}
	stage("complete-persists-and-requests-owned-notification-removal")
}
