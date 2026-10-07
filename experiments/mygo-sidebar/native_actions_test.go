package main

import (
	"github.com/egoist/mygo/ui"
	"math"
	"testing"
	"time"
)

func TestFormalDeleteConfirmCancelAndStableIdentity(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme = theme
			v.visual = webTheme(theme)
			u.Frame()
			click := func(label string) {
				t.Helper()
				if err := u.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			click("编辑：整理今天的工作")
			v.draft = "未保存的草稿"
			click("更多操作：确认设计稿")
			click("删除任务")
			if !u.Focused("取消删除") || m.Tasks[1].Deleted {
				t.Fatal("confirmation deleted task or did not focus cancel")
			}
			click("取消删除")
			if v.deleteID != 0 || m.totalCount() != 3 {
				t.Fatal("cancel changed tasks")
			}
			click("删除任务")
			u.Key(0, ui.KeyEscape)
			if v.moreID != 0 || v.editID != 1 || v.draft != "未保存的草稿" || !u.Focused("更多操作：确认设计稿") {
				t.Fatal("Escape cancelled unrelated draft")
			}
			click("更多操作：确认设计稿")
			click("删除任务")
			click("确认删除")
			snapshot := v.service.List()
			if m.totalCount() != 2 || len(snapshot.Tasks) != 2 || snapshot.Tasks[1].ID != 3 || v.editID != 1 || v.draft != "未保存的草稿" {
				t.Fatal("delete shifted IDs or lost another draft")
			}
			if _, err := v.service.Update(2, "已删除", 0, 1); err == nil {
				t.Fatal("deleted task still editable")
			}
			if err := v.service.Open(2); err == nil {
				t.Fatal("deleted task still opens")
			}
			added, err := v.service.AddWithPin("新任务", "", 0, false)
			if err != nil || added.Tasks[2].ID != 4 {
				t.Fatal("reused a deleted ID")
			}
			if m.Tasks[1].Title != "" || m.Tasks[1].Note != "" {
				t.Fatal("deleted payload retained")
			}
			click("更多操作：整理今天的工作")
			click("编辑任务标题")
			if v.moreID != 0 {
				t.Fatal("outside click left menu open")
			}
		})
	}
}

func TestFormalUndoFromMainAndSidebarExpires(t *testing.T) {
	m, v, u := webUITester(t)
	now := time.Date(2026, 10, 7, 16, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	if err := u.Click("完成：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if !m.Tasks[0].Done || m.UndoID != 1 || m.UndoUntil != now.Add(5*time.Second).UnixMilli() || !u.HasText("已完成「整理今天的工作」") {
		t.Fatal("completion undo state missing")
	}
	if err := u.Click("撤销完成"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Done || m.UndoID != 0 {
		t.Fatal("undo failed")
	}
	m.open(0)
	m.complete()
	v.service.revision++
	overlay := &views{m: m, service: v.service}
	stack := ui.NewTester(overlay.stack, stackWidth, m.stackHeight())
	if len(stackInputRegions(m)) != 3 {
		t.Fatal("undo toast has no input region")
	}
	if err := stack.Click("撤销"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Done || m.undoVisible() {
		t.Fatal("desktop undo failed")
	}
	_, _ = v.service.SetDone(1, true, m.Tasks[0].Version)
	now = now.Add(6 * time.Second)
	u.Frame()
	if m.undoVisible() || u.HasText("撤销完成") {
		t.Fatal("expired undo still visible")
	}
	if _, err := v.service.Undo(); err == nil || !m.Tasks[0].Done {
		t.Fatal("expired undo changed completion")
	}
}

func TestFormalSnoozeMenuRestorationAndExpiry(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		for _, duration := range []struct {
			label, value string
			delta        time.Duration
		}{{"30 分钟", "30m", 30 * time.Minute}, {"1 小时", "1h", time.Hour}, {"明天 09:00", "tomorrow", 17 * time.Hour}} {
			t.Run(theme+"/"+duration.value, func(t *testing.T) {
				m := newModel()
				m.UITheme = theme
				now := time.Date(2026, 10, 7, 16, 0, 0, 0, time.Local)
				m.clock = func() time.Time { return now }
				s := testService(m)
				m.open(0)
				v := &views{m: m, service: s}
				u := ui.NewTester(v.card, 320, 390)
				if err := u.Click("稍后"); err != nil {
					t.Fatal(err)
				}
				if !v.snoozing || !u.HasText("明天 09:00") {
					t.Fatal("snooze options missing")
				}
				if err := u.Click("返回操作"); err != nil {
					t.Fatal(err)
				}
				if v.snoozing || m.Tasks[0].SnoozedUntil != 0 {
					t.Fatal("return snoozed task")
				}
				_ = u.Click("稍后")
				if err := u.Click(duration.label); err != nil {
					t.Fatal(err)
				}
				if m.Opened != -1 || m.sidebarVisible(0) || m.Tasks[0].SnoozedUntil != now.Add(duration.delta).UnixMilli() || m.activeCount() != 3 || len(s.List().Tasks) != 3 {
					t.Fatal("snooze removed task or used wrong deadline")
				}
				list := newNativeTasksView(s)
				main := ui.NewTester(list.render, 1120, 778)
				if !main.HasText("暂时隐藏至 " + now.Add(duration.delta).Format("1/2 15:04")) {
					t.Fatal("hidden-until status missing")
				}
				if err := main.Click("现在恢复"); err != nil {
					t.Fatal(err)
				}
				if !m.sidebarVisible(0) || m.Tasks[0].SnoozedUntil != 0 {
					t.Fatal("restore failed")
				}
				_, _ = s.Snooze(1, duration.value, m.Tasks[0].Version)
				now = now.Add(duration.delta - time.Millisecond)
				if m.sidebarVisible(0) {
					t.Fatal("restored too soon")
				}
				now = now.Add(time.Millisecond)
				if !m.sidebarVisible(0) {
					t.Fatal("not restored at expiry")
				}
			})
		}
	}
}

func TestFormalExpiryInvalidatesQueuedCallbacks(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	s := testService(m)
	var wakes []func()
	var delays []time.Duration
	events := 0
	s.schedule = func(d time.Duration, f func()) { delays = append(delays, d); wakes = append(wakes, f) }
	s.changed = func(string) { events++ }
	_, _ = s.Snooze(1, "30m", 1)
	if len(wakes) != 1 || delays[0] != 30*time.Minute {
		t.Fatal("wrong expiry wake")
	}
	_, _ = s.Unsnooze(1, m.Tasks[0].Version)
	revision := s.revision
	count := events
	now = now.Add(30 * time.Minute)
	wakes[0]()
	if s.revision != revision || events != count {
		t.Fatal("obsolete timer applied")
	}
	_, _ = s.Snooze(1, "1h", m.Tasks[0].Version)
	now = now.Add(time.Hour)
	wakes[len(wakes)-1]()
	if !m.sidebarVisible(0) || events != count+2 {
		t.Fatal("expiry did not publish restored regions")
	}
}

func TestFormalLayoutControlsGeometryAndDragOffset(t *testing.T) {
	m, v, u := webUITester(t)
	m.WorkX, m.WorkY, m.WorkWidth, m.WorkHeight = 100, 30, 1600, 1000
	if err := u.Click("桌面位置"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("桌面标签密度"); err != nil {
		t.Fatal(err)
	}
	u.Key(0, ui.KeyEnd)
	u.Key(0, ui.KeyEnter)
	if m.itemHeight() != 56 {
		t.Fatalf("density control inactive: %d", m.itemHeight())
	}
	if err := u.Click("桌面边缘"); err != nil {
		t.Fatal(err)
	}
	u.Key(0, ui.KeyEnd)
	u.Key(0, ui.KeyEnter)
	if m.Side != "right" {
		t.Fatalf("side selector inactive: side=%s status=%s", m.Side, v.status)
	}
	for _, side := range []string{"left", "right"} {
		for _, height := range []int{40, 44, 56} {
			if _, err := v.service.Layout(side, .35, height); err != nil {
				t.Fatal(err)
			}
			r := m.markerRect(2)
			body := m.sidebarCount()*height + (m.sidebarCount()-1)*6
			if r.Height != height || r.Y != 10+2*(height+6) || m.Y+10+body/2 != m.WorkY+350 {
				t.Fatalf("wrong geometry: %+v, y=%d", r, m.Y)
			}
			for _, region := range stackInputRegions(m) {
				if region.rect.Height != height {
					t.Fatal("input density differs from paint")
				}
			}
		}
	}
	if err := u.Click("桌面中心位置"); err != nil {
		t.Fatal(err)
	}
	beforeSlider := m.Offset
	u.Key(0, ui.KeyRight)
	if math.Abs(m.Offset-beforeSlider-.01) > 1e-6 {
		t.Fatalf("slider inactive: %f", m.Offset)
	}
	oldSide, oldOffset, oldHeight := m.Side, m.Offset, m.itemHeight()
	for _, invalid := range []struct {
		side   string
		offset float64
		height int
	}{{"other", .5, 44}, {"left", math.NaN(), 44}, {"right", 2, 44}, {"left", .5, 41}} {
		if _, err := v.service.Layout(invalid.side, invalid.offset, invalid.height); err == nil {
			t.Fatal("invalid layout accepted")
		}
	}
	if m.Side != oldSide || m.Offset != oldOffset || m.itemHeight() != oldHeight {
		t.Fatal("invalid layout mutated state")
	}
	m.startDrag(m.X+7, m.Y+10)
	m.moveDrag(m.X+7, m.Y+110)
	m.endDrag()
	if math.Abs(m.Offset-float64(m.Y-m.WorkY+m.stackHeight()/2)/float64(m.WorkHeight)) > 1e-6 {
		t.Fatal("drag did not update centre position")
	}
}

func TestFormalCardSharedPresenceProtectsReentryAndEditing(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	m.open(0)
	v := &views{m: m}
	var timers []func()
	v.scheduleClose = func(d time.Duration, f func()) { timers = append(timers, f) }
	v.beginSession(0)
	v.presence(false, false)
	if len(timers) != 1 || v.session.CloseAt != now.Add(500*time.Millisecond) {
		t.Fatal("leave bridge missing")
	}
	old := timers[0]
	now = now.Add(200 * time.Millisecond)
	v.presence(false, true)
	now = now.Add(time.Second)
	old()
	if m.Opened != 0 {
		t.Fatal("old leave closed after entering card")
	}
	v.presence(false, false)
	m.edit()
	now = now.Add(time.Second)
	timers[len(timers)-1]()
	if m.Opened != 0 || !m.Editing {
		t.Fatal("auto-close discarded edit")
	}
	m.cancel()
	v.notify("cancel")
	now = now.Add(501 * time.Millisecond)
	timers[len(timers)-1]()
	if m.Opened != -1 {
		t.Fatal("passive card did not close after cancel and absence")
	}
	m.open(0)
	v.beginSession(0)
	v.presence(false, false)
	old = timers[len(timers)-1]
	v.closeCard()
	m.open(1)
	v.beginSession(1)
	now = now.Add(time.Second)
	old()
	if m.Opened != 1 {
		t.Fatal("reopening did not invalidate old close timer")
	}
}

func TestFormalCardAnchorsBothSidesAndUnclampedOrigin(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	m.WorkX, m.WorkY, m.WorkWidth, m.WorkHeight = 100, 30, 1600, 1000
	m.X, m.Y = 100, 400
	m.open(0)
	x, y := m.cardAnchor(0)
	if x != 408 || y != 398 {
		t.Fatalf("left anchor: %d,%d", x, y)
	}
	m.Side, m.X = "right", 1388
	x, y = m.cardAnchor(0)
	if x != 1072 || y != 398 {
		t.Fatalf("right anchor: %d,%d", x, y)
	}
	m.Y = 900
	_, y = m.cardAnchor(0)
	if y != 898 {
		t.Fatal("anchor was prematurely clamped; cancel would move it")
	}
}

func TestFormalCardSnoozeNoteRemainsVisible(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	m.open(0)
	v := &views{m: m}
	u := ui.NewTester(v.card, 320, 390)
	_ = u.Click("稍后")
	u.SetSize(320, m.CardReadHeight)
	u.Frame()
	note, _ := u.Find(m.Tasks[0].Note)
	option, _ := u.Find("30 分钟")
	title, _ := u.Find(m.Tasks[0].Title)
	t.Logf("height=%d note=%+v title=%+v option=%+v", m.CardReadHeight, note, title, option)
	if note.Y+note.H > option.Y-12 {
		t.Fatal("snooze footer clips note")
	}
}
