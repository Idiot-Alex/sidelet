package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestNewTaskDraftDebounce(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	v := newNativeTasksView(s)
	writes, write := 0, p.writeDraft
	p.writeDraft = func(data []byte) error { writes++; return write(data) }
	// Frame timestamps need not strictly increase between fast input events.
	// Test the scheduler with explicit time, including two edits in one tick.
	now := time.Unix(10000, 0)
	v.newTitle = "你"
	if delay, _ := v.saveNewTaskDraftWhenDue(now); delay != newTaskDraftDelay || writes != 0 {
		t.Fatal("first edit did not schedule debounce", delay, writes)
	}
	v.newTitle = "你好"
	if delay, _ := v.saveNewTaskDraftWhenDue(now); delay != newTaskDraftDelay || m.newTaskDraft.Title != "你好" || writes != 0 {
		t.Fatal("same-tick edit lost latest input", delay, writes)
	}
	v.newTitle = "你好，明天继续"
	lastEdit := now.Add(300 * time.Millisecond)
	if delay, _ := v.saveNewTaskDraftWhenDue(lastEdit); delay != newTaskDraftDelay {
		t.Fatal("editing did not reset debounce", delay)
	}
	if delay, _ := v.saveNewTaskDraftWhenDue(now.Add(newTaskDraftDelay)); delay != 300*time.Millisecond || writes != 0 {
		t.Fatal("old timer saved the later edit too early", delay, writes)
	}
	if delay, _ := v.saveNewTaskDraftWhenDue(lastEdit.Add(newTaskDraftDelay - time.Millisecond)); delay != time.Millisecond || writes != 0 {
		t.Fatal("draft wrote before deadline", delay, writes)
	}
	v.saveNewTaskDraftWhenDue(lastEdit.Add(newTaskDraftDelay))
	if writes != 1 || p.saved.NewTaskDraft == nil || p.saved.NewTaskDraft.Title != "你好，明天继续" {
		t.Fatalf("committed input not saved: writes=%d draft=%+v", writes, p.saved.NewTaskDraft)
	}
	v.saveNewTaskDraftWhenDue(now.Add(time.Hour))
	if writes != 1 {
		t.Fatal("unchanged draft wrote again")
	}
	v.newTitle = ""
	v.saveNewTaskDraftWhenDue(now.Add(2 * time.Hour))
	if writes != 1 {
		t.Fatal("empty draft bypassed debounce")
	}
	v.saveNewTaskDraftWhenDue(v.draftSaveAt)
	if writes != 2 || p.saved.NewTaskDraft != nil {
		t.Fatal("empty form did not clear persisted draft")
	}
}

func TestNewTaskDraftIMEAndClearing(t *testing.T) {
	m, s, p := profileFixture(t, t.TempDir())
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 1120, 1100)
	u.SetFocused(true)
	if err := u.Click("新建任务"); err != nil {
		t.Fatal(err)
	}
	u.Compose("ni", 2)
	if !v.flushNewTaskDraft() || m.newTaskDraft != nil || p.saved.NewTaskDraft != nil {
		t.Fatal("IME preedit persisted")
	}
	u.Type("你")
	u.Type("好，明天继续")
	if !v.flushNewTaskDraft() || p.saved.NewTaskDraft == nil || p.saved.NewTaskDraft.Title != "你好，明天继续" {
		t.Fatal("committed input not saved")
	}
	// A quick-add task is independent and must not consume the main draft.
	if _, err := s.AddWithPin("快捷新增", "", 0, false); err != nil {
		t.Fatal(err)
	}
	if !equalNewTaskDraft(p.saved.NewTaskDraft, m.newTaskDraft) || m.newTaskDraft == nil {
		t.Fatal("quick add consumed main draft")
	}
	u.Command("selectAll")
	u.Key(0, ui.KeyBackspace)
	if !v.flushNewTaskDraft() || v.newTitle != "" || p.saved.NewTaskDraft != nil {
		t.Fatalf("empty form did not clear draft: title=%q draft=%+v", v.newTitle, p.saved.NewTaskDraft)
	}
}

func TestNewTaskDraftRestoresAcrossRestartWhileEditingExistingTask(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			dir := t.TempDir()
			_, s, p := profileFixture(t, dir)
			if _, err := s.Theme(theme); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddWithPin("已有任务", "原备注", 0, false); err != nil {
				t.Fatal(err)
			}
			v := newNativeTasksView(s)
			u := ui.NewTester(v.render, 1120, 1100)
			u.SetFocused(true)
			if err := u.Click("新建任务"); err != nil {
				t.Fatal(err)
			}
			u.Type("跨重启中文草稿")
			if err := u.Click("展开备注"); err != nil {
				t.Fatal(err)
			}
			u.Type("第一行\n第二行")
			if err := u.Click("收起备注"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("截止时间与更多选项"); err != nil {
				t.Fatal(err)
			}
			if err := u.Click("截止年份"); err != nil {
				t.Fatal(err)
			}
			u.Type("2027")
			v.newPriority, v.formPin, v.formRemind, v.formTemporary = "紧急", true, true, true
			u.Frame()
			want := v.captureNewTaskDraft()
			if want.Note != "第一行\n第二行" || want.NoteOpen || want.DeadlineParts[0] != "2027" || want.Due == "" {
				t.Fatalf("fixture is incomplete: %+v", want)
			}
			v.beginEdit(s.snapshot().Tasks[0])
			v.draft, v.formNote, v.formDue = "不要保存的已有任务修改", "编辑中的备注", "2028/01/02 03:04"
			if !v.flushNewTaskDraft() {
				t.Fatal(v.draftSaveError)
			}
			if !equalNewTaskDraft(want, p.saved.NewTaskDraft) {
				t.Fatal("existing task editor overwrote new-task draft")
			}
			p.close()
			m2, s2, _ := profileFixture(t, dir)
			v2 := newNativeTasksView(s2)
			u2 := ui.NewTester(v2.render, 1120, 1100)
			if !equalNewTaskDraft(want, v2.captureNewTaskDraft()) || v2.editID != 0 || m2.Tasks[0].Title != "已有任务" || m2.Tasks[0].Note != "原备注" {
				t.Fatalf("restart restored wrong state: %+v", v2.captureNewTaskDraft())
			}
			if _, ok := u2.Find("截止年份"); !ok {
				t.Fatal("expanded options did not restore")
			}
			v2.webSubmit()
			if !v2.failed || len(m2.Tasks) != 1 || !equalNewTaskDraft(want, m2.newTaskDraft) {
				t.Fatal("invalid partial date consumed draft")
			}
		})
	}
}

func TestNewTaskDraftSaveFailureBlocksQuitAndCanRetry(t *testing.T) {
	dir := t.TempDir()
	_, s, p := profileFixture(t, dir)
	v := newNativeTasksView(s)
	u := ui.NewTester(v.render, 1120, 1100)
	write, attempts := p.writeDraft, 0
	p.writeDraft = func([]byte) error { attempts++; return errors.New("disk full fixture") }
	v.newTitle = "保存失败也不能丢"
	u.Frame()
	v.saveNewTaskDraftWhenDue(v.draftSaveAt)
	v.saveNewTaskDraftWhenDue(time.Now().Add(time.Hour))
	if attempts != 1 || v.draftSaveError == "" || v.newTitle == "" {
		t.Fatal("failure lost draft or retried endlessly")
	}
	h := &nativeApp{service: s, view: v}
	e := &mygo.QuitEvent{}
	h.beforeQuit(e)
	if !e.DefaultPrevented() || h.quitting || h.view == nil || h.flushMainDraft() {
		t.Fatal("failed flush allowed quit/hide/close")
	}
	p.writeDraft = write
	u.SetSize(820, 600)
	retry := visualRect(t, u, "重试保存草稿")
	if retry.Y < 0 || retry.Y+retry.H > 600 {
		t.Fatal("save failure retry is outside the compact window", retry)
	}
	if err := u.Click("重试保存草稿"); err != nil {
		t.Fatal(err)
	}
	if v.draftSaveError != "" || p.saved.NewTaskDraft == nil {
		t.Fatal("retry did not save draft")
	}
	e = &mygo.QuitEvent{}
	h.beforeQuit(e)
	if e.DefaultPrevented() {
		t.Fatal("saved draft prevented quit")
	}
	p.close()
	_, s2, _ := profileFixture(t, dir)
	if newNativeTasksView(s2).newTitle != "保存失败也不能丢" {
		t.Fatal("retry was not durable")
	}
}

func TestNewTaskCreationConsumesDraftOnlyAfterSuccessfulCommit(t *testing.T) {
	dir := t.TempDir()
	m, s, p := profileFixture(t, dir)
	v := newNativeTasksView(s)
	v.newTitle, v.formNote = "只添加一次", "重启后不重复恢复"
	if !v.flushNewTaskDraft() {
		t.Fatal(v.draftSaveError)
	}
	write := p.write
	p.write = func(string, []byte) error { return errors.New("failed transaction fixture") }
	v.webSubmit()
	if !v.failed || len(m.Tasks) != 0 || m.newTaskDraft == nil || v.newTitle == "" || p.saved.NewTaskDraft == nil {
		t.Fatal("failed creation consumed draft")
	}
	p.write = write
	v.webSubmit()
	if v.failed || len(m.Tasks) != 1 || m.newTaskDraft != nil || v.newTitle != "" || p.saved.NewTaskDraft != nil {
		t.Fatal("successful creation left draft behind")
	}
	v.saveNewTaskDraftWhenDue(time.Now().Add(time.Hour))
	p.close()
	m2, s2, _ := profileFixture(t, dir)
	if len(m2.Tasks) != 1 || newNativeTasksView(s2).newTitle != "" {
		t.Fatal("restart duplicated task or restored consumed draft")
	}
}

func TestNewTaskDraftOversizeDoesNotBlockOtherTaskSaves(t *testing.T) {
	m, s, _ := profileFixture(t, t.TempDir())
	v := newNativeTasksView(s)
	v.newTitle = "原有效草稿"
	if !v.flushNewTaskDraft() {
		t.Fatal(v.draftSaveError)
	}
	v.formNote = strings.Repeat("x", 1<<20)
	if v.flushNewTaskDraft() || v.formNote == "" || m.newTaskDraft.Note != "" {
		t.Fatal("oversize draft lost input or poisoned persisted model")
	}
	if _, err := s.AddWithPin("其他任务仍可保存", "", 0, false); err != nil {
		t.Fatal(err)
	}
	v.formNote = "缩短后可重试"
	if !v.flushNewTaskDraft() || m.newTaskDraft.Note != "缩短后可重试" {
		t.Fatal("corrected draft failed to save")
	}
}
