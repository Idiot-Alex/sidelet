package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"sidelet/internal/reminder"
)

func TestScheduleDraftValidationPreservationAndClear(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme = theme
			v.visual = webTheme(theme)
			now := time.Date(2026, 10, 7, 14, 0, 0, 0, time.Local)
			m.clock = func() time.Time { return now }
			u.SetSize(1120, 1100)
			due := now.Add(time.Hour).UnixMilli() + 1234
			_, err := v.service.SaveTask(1, m.Tasks[0].Title, "备注", 2, true, due, true, true, 1)
			if err != nil {
				t.Fatal(err)
			}
			u.Frame()
			if err := u.Click("编辑：整理今天的工作"); err != nil {
				t.Fatal(err)
			}
			if !v.formRemind || !v.formTemporary || v.originalDue != due || !v.optionsOpen {
				t.Fatal("schedule draft not loaded")
			}
			if !u.HasText("即将到期 · "+time.UnixMilli(due).Format("1/2 15:04")) && !u.HasText(time.UnixMilli(due).Format("1/2 15:04")) {
				t.Fatal("deadline missing from task")
			}
			v.draft = "改标题保留精确时间"
			if err := u.Click("保存修改"); err != nil {
				t.Fatal(err)
			}
			if m.Tasks[0].DueAt != due || !m.Tasks[0].Remind || !m.Tasks[0].Temporary || v.formRemind || v.formTemporary || v.formDue != "" {
				t.Fatal("save lost sub-minute timestamp or leaked new form options")
			}
			_ = u.Click("编辑：改标题保留精确时间")
			v.formDue = "2026-02-30T12:00"
			v.draft = "无效日期不能覆盖"
			before := m.Tasks[0]
			if err := u.Click("保存修改"); err != nil {
				t.Fatal(err)
			}
			if !v.failed || m.Tasks[0] != before || v.formDue != "2026-02-30T12:00" {
				t.Fatal("invalid date mutated task or lost draft")
			}
			_ = u.Click("取消编辑")
			_ = u.Click("编辑：改标题保留精确时间")
			if err := u.Click("截止年份"); err != nil {
				t.Fatal(err)
			}
			u.Command("selectAll")
			u.Key(0, ui.KeyBackspace)
			if v.formDue != "" || v.formRemind {
				t.Fatal("clearing deadline did not disable reminder")
			}
			_ = u.Click("保存修改")
			if m.Tasks[0].DueAt != 0 || m.Tasks[0].Remind || !m.Tasks[0].Temporary {
				t.Fatal("clear changed unrelated options")
			}
			if _, err := v.service.SaveTask(1, "标题", "", 0, false, 0, true, false, m.Tasks[0].Version); err == nil {
				t.Fatal("reminder without due accepted")
			}
			_ = u.Click("编辑：改标题保留精确时间")
			v.formDue = "2026-10-08T16:00"
			v.formRemind = true
			v.formTemporary = true
			v.draft = "冲突草稿"
			_, _ = v.service.UpdateDetails(1, "卡片修改", "最新备注", 0, m.Tasks[0].Version)
			u.Frame()
			_ = u.Click("保存修改")
			if !v.failed || v.draft != "冲突草稿" || v.formDue != "2026-10-08T16:00" || !v.formRemind || m.Tasks[0].Title != "卡片修改" {
				t.Fatal("conflict overwrote task or reset schedule draft")
			}
		})
	}
}
func TestDeadlinePickerApplyClearAndNestedEscape(t *testing.T) {
	m, v, u := webUITester(t)
	m.clock = func() time.Time { return time.Date(2026, 10, 7, 16, 0, 0, 0, time.Local) }
	u.SetSize(1120, 1100)
	_ = u.Click("截止时间与更多选项")
	if err := u.Click("选择截止日期与时间"); err != nil {
		t.Fatal(err)
	}
	if !v.datePickerOpen || !u.HasText("截止日期与时间") {
		t.Fatal("picker did not open")
	}
	if err := u.Click("选择截止日期"); err != nil {
		t.Fatal(err)
	}
	u.Key(0, ui.KeyRight)
	u.Key(0, ui.KeyEnter)
	if err := u.Click("截止时间小时"); err != nil {
		t.Fatal(err)
	}
	u.Key(0, ui.KeyUp)
	if err := u.Click("确定截止时间"); err != nil {
		t.Fatal(err)
	}
	if v.formDue != "2026/10/08 18:00" || v.datePickerOpen {
		t.Fatalf("date/time picker not applied: %s", v.formDue)
	}
	if err := u.Click("提醒我"); err != nil {
		t.Fatal(err)
	}
	_ = u.Click("选择截止日期与时间")
	u.Key(0, ui.KeyEscape)
	if v.datePickerOpen || v.formDue == "" || !v.formRemind {
		t.Fatalf("Escape changed picker draft: open=%v due=%s remind=%v", v.datePickerOpen, v.formDue, v.formRemind)
	}
	_ = u.Click("选择截止日期与时间")
	_ = u.Click("清除截止时间")
	if v.formDue != "" || v.formRemind || v.datePickerOpen {
		t.Fatal("picker clear failed")
	}
}
func TestDueStatesAndCardSaveKeepTaskSchedule(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	s := testService(m)
	_, _ = s.SaveTask(1, m.Tasks[0].Title, m.Tasks[0].Note, 2, true, now.Add(31*time.Minute).UnixMilli(), true, true, 1)
	if dueLabel(m.Tasks[0], now) != "" || dueLabel(m.Tasks[0], now.Add(time.Minute)) != "即将到期" || dueLabel(m.Tasks[0], now.Add(31*time.Minute)) != "已逾期" {
		t.Fatal("deadline boundaries differ from formal UI")
	}
	m.open(0)
	m.edit()
	m.Draft = "卡片改标题"
	m.DraftNote = "卡片改备注"
	if !m.save() {
		t.Fatal(m.EditError)
	}
	if !m.Tasks[0].Remind || !m.Tasks[0].Temporary || m.Tasks[0].DueAt != now.Add(31*time.Minute).UnixMilli() {
		t.Fatal("card edit lost schedule")
	}
	m.Hover = 0
	v := &views{m: m}
	stack := ui.NewTester(v.stack, 312, m.stackHeight())
	if !stack.HasText("00:47") && !stack.HasText(time.UnixMilli(m.Tasks[0].DueAt).In(now.Location()).Format("15:04")) {
		t.Fatal("sidebar deadline missing")
	}
	card := ui.NewTester(v.card, 320, 390)
	if !card.HasText(dueText(m.Tasks[0], now)) {
		t.Fatal("card deadline missing")
	}
	m.Tasks[0].SnoozedUntil = now.Add(time.Hour).UnixMilli()
	if dueLabel(m.Tasks[0], now.Add(31*time.Minute)) != "" {
		t.Fatal("hidden task marked overdue")
	}
	m.Tasks[0].SnoozedUntil = 0
	m.Tasks[0].Done = true
	if dueLabel(m.Tasks[0], now.Add(time.Hour)) != "" {
		t.Fatal("completed task marked overdue")
	}
}

func TestDeadlineCalendarFitsLaptopWindowAndProtectsEditor(t *testing.T) {
	for _, size := range [][2]int{{1120, 800}, {820, 600}} {
		m, v, u := webUITester(t)
		m.clock = func() time.Time { return time.Date(2026, 10, 7, 16, 0, 0, 0, time.Local) }
		u.SetSize(size[0], size[1])
		_ = u.Click("编辑：整理今天的工作")
		_ = u.Click("截止时间与更多选项")
		_ = u.Click("选择截止日期与时间")
		_ = u.Click("选择截止日期")
		r, found := u.Find("2026年11月8日")
		if !found || r.Y < 0 || r.Y+r.H > float32(size[1]) {
			t.Fatalf("calendar clipped at %v: %+v found=%v", size, r, found)
		}
		u.Key(0, ui.KeyEscape)
		if !v.datePickerOpen || v.editID != 1 {
			t.Fatal("nested Escape cancelled editor")
		}
		u.Key(0, ui.KeyEscape)
		if v.datePickerOpen || v.editID != 1 {
			t.Fatal("picker Escape cancelled editor")
		}
	}
}

func TestReminderShutdownRemovesOnlyAllOwnedGenerations(t *testing.T) {
	m, s, n, b := reminderFixture(t)
	_, _ = s.SaveTask(1, "提醒任务", "", 0, true, 900000, true, false, 1)
	_ = runReminderJob(t, n)
	_, _ = s.SaveTask(1, "新提醒", "", 0, true, 2000000, true, false, m.Tasks[0].Version)
	n.close()
	if !n.stopped.Load() || len(b.removed) != 2 || b.removed[0] != "test.1" || b.removed[1] != "test.2" {
		t.Fatal("shutdown did not remove owned superseded generation", b.removed)
	}
	n.request()
	if len(n.jobs) != 1 {
		t.Fatal("shutdown queued more work")
	}
}
func TestTemporaryCleanupAfterOwnCompletionAndUndo(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	s := testService(m)
	_, _ = s.SaveTask(1, "临时任务", "", 0, true, 0, false, true, 1)
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	now = now.Add(4 * time.Second)
	_, _ = s.Undo()
	now = now.Add(10 * time.Second)
	if m.cleanup() || m.Tasks[0].Deleted || m.Tasks[0].CompletedAt != 0 {
		t.Fatal("undo did not protect temporary task")
	}
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	now = now.Add(time.Second)
	_, _ = s.SetDone(2, true, m.Tasks[1].Version)
	now = now.Add(4 * time.Second)
	if m.cleanup() {
		t.Fatal("removed at exact undo boundary")
	}
	now = now.Add(time.Millisecond)
	if !m.cleanup() || !m.Tasks[0].Deleted || m.UndoID != 2 || m.Tasks[1].Deleted {
		t.Fatal("cleanup used latest UndoUntil instead of own completion")
	}
	if s.List().Tasks[0].ID != 2 || m.totalCount() != 2 {
		t.Fatal("cleanup shifted identities")
	}
}
func TestScheduleDeadlineAndCleanupUseSingleWake(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	s := testService(m)
	var callbacks []func()
	var delays []time.Duration
	s.schedule = func(d time.Duration, f func()) { delays = append(delays, d); callbacks = append(callbacks, f) }
	_, _ = s.SaveTask(1, "定时任务", "", 0, true, now.Add(time.Hour).UnixMilli(), false, true, 1)
	if delays[len(delays)-1] != 30*time.Minute {
		t.Fatal("soon state has no wake")
	}
	now = now.Add(30 * time.Minute)
	callbacks[len(callbacks)-1]()
	if delays[len(delays)-1] != 30*time.Minute {
		t.Fatal("deadline has no next wake")
	}
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	if delays[len(delays)-1] != 5001*time.Millisecond {
		t.Fatal("cleanup timer not coalesced with undo")
	}
	now = now.Add(5001 * time.Millisecond)
	callbacks[len(callbacks)-1]()
	if !m.Tasks[0].Deleted {
		t.Fatal("timer failed to clean temporary task")
	}
}
func TestLocalDeadlineParsingDSTAndPrecision(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"2026-03-08T02:30", "2026-02-30T12:00", "invalid"} {
		if _, err := parseDue(text, 0, "", zone); err == nil {
			t.Fatal("invalid local date accepted", text)
		}
	}
	original := time.Date(2026, 11, 1, 1, 30, 12, 345000000, zone).UnixMilli()
	text := localDue(original, zone)
	if got, err := parseDue(text, original, text, zone); err != nil || got != original {
		t.Fatal("unchanged ambiguous time lost exact timestamp")
	}
	if at, err := parseDue("2026/10/07 16:00", 0, "", zone); err != nil || time.UnixMilli(at).In(zone).Hour() != 16 {
		t.Fatal("local entry parse failed")
	}
}

type testNotifications struct {
	state              reminder.State
	sent, removed      []string
	fail               bool
	onState, onDeliver func()
}

func (b *testNotifications) State() (reminder.State, error) {
	if b.onState != nil {
		f := b.onState
		b.onState = nil
		f()
	}
	return b.state, nil
}
func (b *testNotifications) Deliver(id, title, body string) error {
	if b.fail {
		return errors.New("system unavailable")
	}
	b.sent = append(b.sent, id)
	b.state.Delivered = append(b.state.Delivered, id)
	if b.onDeliver != nil {
		f := b.onDeliver
		b.onDeliver = nil
		f()
	}
	return nil
}
func (b *testNotifications) Remove(ids []string) error {
	b.removed = append(b.removed, ids...)
	return nil
}
func (b *testNotifications) RequestPermission(done func()) {
	b.state.Authorization = "authorized"
	done()
}
func (b *testNotifications) Watch(func()) func() { return func() {} }
func runReminderJob(t *testing.T, n *nativeReminders) error {
	t.Helper()
	n.request()
	job := <-n.jobs
	repo := &receiptRepo{rows: job.rows, accepted: map[int64]int64{}}
	result, err := reminder.Sync(repo, liveNotificationSystem{n}, n.prefix, job.now)
	n.accept(job, repo.accepted, result, err)
	return err
}
func reminderFixture(t *testing.T) (*model, *TasksService, *nativeReminders, *testNotifications) {
	t.Helper()
	m := newModel()
	m.UITheme = "mac"
	m.clock = func() time.Time { return time.Unix(1000, 0) }
	s := testService(m)
	backend := &testNotifications{state: reminder.State{Authorization: "authorized"}}
	n := newNativeReminders(s, backend, "test.")
	s.notices = n
	return m, s, n, backend
}
func TestReminderDeniedRecoveryGenerationsAndAcknowledgement(t *testing.T) {
	m, s, n, b := reminderFixture(t)
	b.state.Authorization = "denied"
	_, _ = s.SaveTask(1, "提醒任务", "", 0, true, m.now().Add(-time.Minute).UnixMilli(), true, false, 1)
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 0 || m.Tasks[0].ReminderSentAt != 0 || s.notificationAuthorization != "denied" || !strings.Contains(s.notificationStatus, "未获允许") {
		t.Fatal("denied reminder consumed")
	}
	b.state.Authorization = "authorized"
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt == 0 {
		t.Fatal("permission recovery not delivered")
	}
	b.state.Delivered = nil
	_ = runReminderJob(t, n)
	if len(b.sent) != 1 {
		t.Fatal("cleared OS notification resent")
	}
	first := n.rows[1].ID
	_, _ = s.UpdateDetails(1, "修改标题", "备注", 0, m.Tasks[0].Version)
	_ = runReminderJob(t, n)
	if n.rows[1].ID != first || len(b.sent) != 1 {
		t.Fatal("title-only edit regenerated reminder")
	}
	_, _ = s.Snooze(1, "30m", m.Tasks[0].Version)
	if n.rows[1].ID == first || m.Tasks[0].ReminderSentAt != 0 || n.rows[1].At != m.Tasks[0].SnoozedUntil {
		t.Fatal("snooze did not defer generation")
	}
}
func TestReminderStaleDeliveryAndReceiptDoNotConsumeReplacement(t *testing.T) {
	m, s, n, b := reminderFixture(t)
	_, _ = s.SaveTask(1, "旧提醒", "", 0, true, 900000, true, false, 1)
	b.onState = func() { _, _ = s.SaveTask(1, "替代提醒", "", 0, true, 2000000, true, false, m.Tasks[0].Version) }
	if err := runReminderJob(t, n); !errors.Is(err, errReminderSuperseded) || len(b.sent) != 0 || m.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("stale snapshot delivered")
	}
	_, _ = s.SaveTask(1, "再次到期", "", 0, true, 900000, true, false, m.Tasks[0].Version)
	b.onDeliver = func() {
		_, _ = s.SaveTask(1, "发送期间替换", "", 0, true, 3000000, true, false, m.Tasks[0].Version)
	}
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	if len(b.sent) != 1 || m.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("old acceptance consumed new generation")
	}
	if err := runReminderJob(t, n); err != nil {
		t.Fatal(err)
	}
	if len(b.removed) != 1 || b.removed[0] != b.sent[0] {
		t.Fatal("obsolete notification not removed")
	}
}
func TestReminderFailureCompletionUndoAndScopedRemoval(t *testing.T) {
	m, s, n, b := reminderFixture(t)
	_, _ = s.SaveTask(1, "提醒任务", "", 0, true, 900000, true, false, 1)
	b.fail = true
	if err := runReminderJob(t, n); err == nil || m.Tasks[0].ReminderSentAt != 0 || s.nextReminder != m.now().Add(time.Minute) {
		t.Fatal("failed delivery consumed or left no retry")
	}
	b.fail = false
	_ = runReminderJob(t, n)
	first := b.sent[0]
	b.state.Delivered = append(b.state.Delivered, "other-profile.1")
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	_ = runReminderJob(t, n)
	if len(b.removed) != 1 || b.removed[0] != first {
		t.Fatal("completion cleanup touched another namespace")
	}
	_, _ = s.Undo()
	_ = runReminderJob(t, n)
	if len(b.sent) != 1 {
		t.Fatal("undo resent accepted reminder")
	}
	if _, err := s.SaveTask(1, "关闭提醒", "", 0, true, m.Tasks[0].DueAt, false, false, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if len(n.rows) != 0 || m.Tasks[0].ReminderSentAt != 0 {
		t.Fatal("disable retained reminder")
	}
}
func TestReminderCleanupDoesNotRemoveUndoneLiveGeneration(t *testing.T) {
	m, s, n, b := reminderFixture(t)
	_, _ = s.SaveTask(1, "提醒任务", "", 0, true, 900000, true, false, 1)
	_ = runReminderJob(t, n)
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	b.onState = func() { _, _ = s.Undo() }
	_ = runReminderJob(t, n)
	if len(b.removed) != 0 || m.Tasks[0].Done {
		t.Fatal("old cleanup removed undone reminder")
	}
}
