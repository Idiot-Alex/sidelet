package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
	"sidelet/internal/reminder"
	"sidelet/internal/settings"
)

func TestPreferencesRoundTripDoesNotMoveExistingGroup(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	_, _ = s.AddWithPin("已固定任务", "备注", 2, true)
	_, _ = s.Layout("left", .62, 56)
	value := m.Preferences
	value.Edge = settings.Edge{DefaultSide: "right", DefaultDensity: "compact"}
	value.Startup.ShowMainWindow = true
	value.Appearance.ShowDockIcon = false
	s.applyDock = func(bool) error { return nil }
	if _, err := s.SavePreferences(value); err != nil {
		t.Fatal(err)
	}
	if m.Side != "left" || m.Offset != .62 || m.ItemHeight != 56 || m.Tasks[0].Version != 1 {
		t.Fatal("preference changed existing task or layout")
	}
	want := p.saved
	p.close()
	restored, _, p2 := profileFixture(t, dir)
	if !reflect.DeepEqual(want, p2.saved) || restored.Preferences != value || !startupMainVisible(restored, false, false, false) {
		t.Fatal("preferences not restored")
	}
	if _, err := s.Theme("graphite"); err == nil {
		t.Fatal("closed profile accepted theme")
	}
}

func TestProfileSchemaOneMigratesWithoutLosingLegacyState(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	_, _ = s.SaveTask(0, "旧资料任务", "原备注", 3, true, 900000, true, false, 0)
	_, _ = s.Theme("graphite")
	_, _ = s.Layout("left", .7, 40)
	legacy := p.saved
	legacy.Schema, legacy.Preferences = 1, settings.Value{}
	data, _ := json.Marshal(legacy)
	// Real schema 1 has no preferences field.
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	delete(fields, "preferences")
	data, _ = json.Marshal(fields)
	p.close()
	if err := os.WriteFile(p.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	restored, _, p2 := profileFixture(t, dir)
	if p2.saved.Schema != 2 || !restored.Preferences.Startup.ShowMainWindow || restored.Preferences.Appearance.Theme != "graphite" || !restored.Preferences.Appearance.ShowDockIcon {
		t.Fatal("legacy defaults wrong")
	}
	if !reflect.DeepEqual(restored.Tasks, m.Tasks) || restored.Side != "left" || restored.Offset != .7 || restored.ItemHeight != 40 || p2.saved.ID != legacy.ID || restored.ReminderGeneration != legacy.ReminderGeneration {
		t.Fatal("migration lost existing data")
	}
	upgraded, _ := os.ReadFile(p.path)
	if bytes.Equal(upgraded, data) {
		t.Fatal("migration was not committed")
	}
}

func TestProfileInvalidPreferencesPreserveFile(t *testing.T) {
	for _, kind := range []string{"version", "edge", "density", "theme"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			_, _, p := profileFixture(t, dir)
			state := p.saved
			switch kind {
			case "version":
				state.Preferences.Version = 99
			case "edge":
				state.Preferences.Edge.DefaultSide = "invalid"
			case "density":
				state.Preferences.Edge.DefaultDensity = "invalid"
			case "theme":
				state.Preferences.Appearance.Theme = "graphite"
			}
			p.close()
			data, _ := json.Marshal(state)
			_ = os.WriteFile(p.path, data, 0600)
			m := newModel()
			m.UITheme = "mac"
			if _, err := openProfile(dir, m); err == nil {
				t.Fatal("invalid preferences accepted")
			}
			after, _ := os.ReadFile(p.path)
			if !bytes.Equal(data, after) {
				t.Fatal("invalid preferences overwritten")
			}
		})
	}
}

func TestDockPreferenceFailureCompensatesBeforePublishing(t *testing.T) {
	for _, kind := range []string{"save", "apply", "rollback", "unavailable", "drag", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			m, s, p := profileFixture(t, t.TempDir())
			before := p.saved
			data, _ := os.ReadFile(p.path)
			calls := []bool{}
			s.applyDock = func(value bool) error {
				calls = append(calls, value)
				if kind == "apply" && !value || kind == "rollback" && value {
					return errors.New("native failure")
				}
				return nil
			}
			if kind == "save" || kind == "rollback" {
				p.write = func(string, []byte) error { return errors.New("disk full") }
			}
			if kind == "unavailable" {
				s.applyDock = nil
			}
			if kind == "drag" {
				m.startDrag(0, 0)
			}
			value := m.Preferences
			value.Appearance.ShowDockIcon = false
			if kind == "invalid" {
				value.Edge.DefaultSide = "invalid"
			}
			if _, err := s.SavePreferences(value); err == nil {
				t.Fatal("failure accepted")
			}
			if m.Preferences != before.Preferences || m.UITheme != before.Theme || s.revision != 0 {
				t.Fatal("failed preference published")
			}
			after, _ := os.ReadFile(p.path)
			if !bytes.Equal(data, after) {
				t.Fatal("file changed after failure")
			}
			want := []bool{}
			if kind == "save" || kind == "apply" || kind == "rollback" {
				want = []bool{false, true}
			}
			if !slices.Equal(calls, want) {
				t.Fatalf("native compensation %v, want %v", calls, want)
			}
		})
	}
}

func TestStartupPreferenceAndFreshGroupDefaults(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	m.Preferences.Edge = settings.Edge{DefaultSide: "left", DefaultDensity: "relaxed"}
	p, err := openProfile(t.TempDir(), m)
	if err != nil {
		t.Fatal(err)
	}
	defer p.close()
	if m.Side != "left" || m.ItemHeight != 56 {
		t.Fatal("fresh group ignored defaults")
	}
	for _, row := range []struct{ fresh, show, force, hide, want bool }{
		{true, false, false, false, true}, {false, false, false, false, false}, {false, true, false, false, true}, {false, false, true, false, true}, {true, true, true, true, false},
	} {
		p.fresh = row.fresh
		m.Preferences.Startup.ShowMainWindow = row.show
		if got := startupMainVisible(m, row.force, row.hide, false); got != row.want {
			t.Fatalf("startup %+v got %v", row, got)
		}
	}
}

func TestQuietHidesRegionsAndRenderingWithoutMutatingTasks(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	for range 10 {
		_, _ = s.AddWithPin("桌面任务", "备注", 0, true)
	}
	m.WorkWidth, m.WorkHeight = 1728, 1000
	m.positionStack()
	x, y := m.X, m.Y
	m.startDrag(x, y)
	m.moveDrag(x+100, y+80)
	before := p.saved
	data, _ := os.ReadFile(p.path)
	if _, err := s.SetQuiet(true); err != nil {
		t.Fatal(err)
	}
	if !m.Quiet || m.Dragging || m.Arranging || m.cardOpen() || m.X != x || m.Y != y || len(stackInputRegions(m)) != 0 {
		t.Fatal("quiet left a live interaction")
	}
	direct, overflow := m.sidebarItems()
	if len(direct)+len(overflow) != 0 || len(m.eligibleIDs()) != 10 {
		t.Fatal("quiet lost logical tasks or showed overflow")
	}
	u := ui.NewTester((&views{m: m}).stack, stackWidth, m.stackHeight())
	img := u.Image()
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.RGBAAt(x, y).A != 0 {
				t.Fatal("quiet painted sidebar")
			}
		}
	}
	if _, err := s.Arrange(true); err == nil {
		t.Fatal("quiet allowed arrange")
	}
	m.open(0)
	if m.cardOpen() {
		t.Fatal("quiet opened card")
	}
	m.startDrag(x, y)
	if m.Dragging {
		t.Fatal("stale pointer started a drag in quiet mode")
	}
	if m.openOverflow() {
		t.Fatal("quiet opened overflow")
	}
	after, _ := os.ReadFile(p.path)
	if !bytes.Equal(data, after) || !reflect.DeepEqual(before, p.saved) {
		t.Fatal("quiet wrote task data")
	}
	if _, err := s.SetQuiet(false); err != nil || len(stackInputRegions(m)) != 9 {
		t.Fatal("restore did not restore overflow regions", err)
	}
	_, _ = s.SetQuiet(true)
	dir := p.path[:len(p.path)-len("profile.json")]
	p.close()
	restored, _, _ := profileFixture(t, dir)
	if restored.Quiet || restored.sidebarCount() != 10 {
		t.Fatal("quiet persisted across restart")
	}
}

func TestFormalQuietAndPreferencesUIKeepsMainDraft(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			_, _ = v.service.Theme(theme)
			v.visual = webTheme(theme)
			if err := u.Click("编辑：整理今天的工作"); err != nil {
				t.Fatal(err)
			}
			u.Command("selectAll")
			u.Type("保留主窗口草稿")
			if err := u.Click("安静模式"); err != nil {
				t.Fatal(err)
			}
			if !m.Quiet || v.draft != "保留主窗口草稿" || !u.HasText("恢复显示") {
				t.Fatal("quiet lost main editor")
			}
			if err := u.Click("恢复显示"); err != nil {
				t.Fatal(err)
			}
			if m.Quiet || v.draft != "保留主窗口草稿" {
				t.Fatal("restore lost draft")
			}
			if err := u.Click("设置"); err != nil {
				t.Fatal(err)
			}
			u.Scroll(600, 600, 0, 400)
			focus := func(label string) {
				for i := 0; i < 30 && !u.Focused(label); i++ {
					u.Key(0, ui.KeyTab)
				}
				if !u.Focused(label) {
					t.Fatalf("cannot reach %s with keyboard", label)
				}
			}
			focus("启动时显示主窗口")
			u.Key(0, ui.KeySpace)
			if !m.Preferences.Startup.ShowMainWindow {
				t.Fatal("startup switch did not commit")
			}
			focus("默认屏幕边缘")
			u.Key(0, ui.KeySpace)
			u.Key(0, ui.KeyHome)
			u.Key(0, ui.KeyEnter)
			if m.Preferences.Edge.DefaultSide != "left" {
				t.Fatal("default edge did not save")
			}
			focus("默认标签密度")
			u.Key(0, ui.KeySpace)
			u.Key(0, ui.KeyEnd)
			u.Key(0, ui.KeyEnter)
			if m.Preferences.Edge.DefaultDensity != "relaxed" || m.Side != "left" || m.itemHeight() != 44 {
				t.Fatal("default density missing or altered current layout")
			}
		})
	}
}

func TestQuietDoesNotSuppressSystemRemindersOrCreateDuplicates(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	_, _ = s.SaveTask(0, "安静模式中的提醒", "", 0, true, 900000, true, false, 0)
	b := &testNotifications{state: reminder.State{Authorization: "authorized"}}
	n := newNativeReminders(s, b, "sidelet-mygo.profile."+p.saved.ID+".")
	_, _ = s.SetQuiet(true)
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt == 0 || !m.Quiet || len(stackInputRegions(m)) != 0 {
		t.Fatal("quiet stopped reminder or exposed labels")
	}
	_, _ = s.SetQuiet(false)
	if err := runReminderJob(t, n); err != nil || len(b.sent) != 1 {
		t.Fatal("restoring labels duplicated reminder", err)
	}
}
