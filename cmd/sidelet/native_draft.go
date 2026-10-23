package main

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const newTaskDraftDelay = 600 * time.Millisecond

// Only committed input values are stored, never IME preedit text, focus,
// selection, popovers or an existing task's editing/version state.
type persistedNewTaskDraft struct {
	Version       int       `json:"version"`
	Title         string    `json:"title"`
	Note          string    `json:"note"`
	Priority      int       `json:"priority"`
	Pin           bool      `json:"pin"`
	Due           string    `json:"due"`
	Remind        bool      `json:"remind"`
	Temporary     bool      `json:"temporary"`
	NoteOpen      bool      `json:"noteOpen"`
	OptionsOpen   bool      `json:"optionsOpen"`
	DeadlineParts [5]string `json:"deadlineParts"`
	DeadlineSeen  string    `json:"deadlineSeen"`
	DeadlineReady bool      `json:"deadlineReady"`
}

func validateNewTaskDraft(d *persistedNewTaskDraft) error {
	if d == nil {
		return nil
	}
	// Drafts may be incomplete, over task limits or invalid dates. Preserve
	// those values for correction instead of applying task validation here.
	if d.Version != 1 || !validPriority(d.Priority) {
		return errors.New("新建任务草稿版本或字段无效。")
	}
	// Include the formatting used by full profile commits in the size check,
	// so a valid autosave cannot make subsequent task transactions too large.
	data, err := json.MarshalIndent(d, "  ", "  ")
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("草稿内容过大，无法自动保存；请缩短内容后重试。")
	}
	return nil
}

func equalNewTaskDraft(a, b *persistedNewTaskDraft) bool {
	return a == b || a != nil && b != nil && *a == *b
}

func (v *nativeTasksView) captureNewTaskDraft() *persistedNewTaskDraft {
	d := &persistedNewTaskDraft{Version: 1, Title: v.newTitle, Priority: priorityValue(v.newPriority), Pin: v.formPin,
		Note: v.formNote, Due: v.formDue, Remind: v.formRemind, Temporary: v.formTemporary,
		NoteOpen: v.noteOpen, OptionsOpen: v.optionsOpen,
		DeadlineParts: v.deadline.parts, DeadlineSeen: v.deadline.seen, DeadlineReady: v.deadline.ready}
	if v.editID > 0 && v.newTaskDraft != nil {
		held := v.newTaskDraft
		d.Note, d.Due, d.Remind, d.Temporary = held.note, held.due, held.remind, held.temporary
		d.NoteOpen, d.OptionsOpen = held.noteOpen, held.optionsOpen
		d.DeadlineParts, d.DeadlineSeen, d.DeadlineReady = held.deadlineParts, held.deadlineSeen, held.deadlineReady
	}
	if d.DeadlineSeen != d.Due || !d.DeadlineReady {
		d.DeadlineParts, d.DeadlineSeen, d.DeadlineReady = [5]string{}, "", false
	}
	if d.Title == "" && d.Note == "" && d.Due == "" && d.Priority == 0 && !d.Pin && !d.Remind && !d.Temporary {
		return nil
	}
	return d
}

func (v *nativeTasksView) restoreNewTaskDraft(d *persistedNewTaskDraft) {
	if d == nil {
		return
	}
	v.newTitle, v.newPriority, v.formPin = d.Title, priorityName(d.Priority), d.Pin
	v.formNote, v.formDue, v.formRemind, v.formTemporary = d.Note, d.Due, d.Remind, d.Temporary
	v.noteOpen, v.optionsOpen = d.NoteOpen, d.OptionsOpen
	v.deadline = deadlineSegments{parts: d.DeadlineParts, seen: d.DeadlineSeen, ready: d.DeadlineReady, focused: -1}
}

func (v *nativeTasksView) observeNewTaskDraft(now time.Time) error {
	next := v.captureNewTaskDraft()
	if equalNewTaskDraft(next, v.service.m.newTaskDraft) {
		return nil
	}
	if err := validateNewTaskDraft(next); err != nil {
		return err
	}
	v.service.m.newTaskDraft = next
	v.draftSaveAt = now.Add(newTaskDraftDelay)
	v.draftSaveError = ""
	return nil
}

func (p *profile) saveNewTaskDraft(d *persistedNewTaskDraft) error {
	if p.lock == nil {
		return errors.New("资料已关闭。")
	}
	if equalNewTaskDraft(d, p.saved.NewTaskDraft) {
		return nil
	}
	if err := validateNewTaskDraft(d); err != nil {
		return err
	}
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if err = p.writeDraft(data); err != nil {
		return err
	}
	p.saved.NewTaskDraft = d
	return nil
}

func (v *nativeTasksView) draftFailure(err error) bool {
	if err == nil {
		v.draftSaveError = ""
		return true
	}
	log.Printf("new task draft save failed: %v", err)
	v.draftSaveError = "草稿尚未保存，请检查资料目录或缩短内容后重试。关闭窗口或退出前会再次尝试保存。"
	return false
}

func (v *nativeTasksView) flushNewTaskDraft() bool {
	v.draftSaveAt = time.Time{}
	if err := v.observeNewTaskDraft(time.Now()); err != nil {
		return v.draftFailure(err)
	}
	v.draftSaveAt = time.Time{}
	if p := v.service.m.profile; p != nil {
		return v.draftFailure(p.saveNewTaskDraft(v.service.m.newTaskDraft))
	}
	return v.draftFailure(nil)
}

func (v *nativeTasksView) autosaveNewTaskDraft(c *ui.Context) {
	delay, changed := v.saveNewTaskDraftWhenDue(c.Now())
	if delay > 0 {
		c.After(delay)
	}
	if changed {
		c.Invalidate()
	}
}

func (v *nativeTasksView) saveNewTaskDraftWhenDue(now time.Time) (time.Duration, bool) {
	if err := v.observeNewTaskDraft(now); err != nil {
		if v.draftSaveError == "" {
			v.draftFailure(err)
			return 0, true
		}
		return 0, false
	}
	p := v.service.m.profile
	if p == nil || equalNewTaskDraft(v.service.m.newTaskDraft, p.saved.NewTaskDraft) {
		v.draftSaveAt, v.draftSaveError = time.Time{}, ""
		return 0, false
	}
	if v.draftSaveAt.IsZero() {
		return 0, false
	} // Failed saves wait for an edit or explicit retry.
	if delay := v.draftSaveAt.Sub(now); delay > 0 {
		return delay, false
	}
	v.flushNewTaskDraft()
	return 0, true
}

func (v *nativeTasksView) webDraftSaveError(c *ui.Context) {
	if v.draftSaveError == "" {
		return
	}
	ui.Column(c.Key("draft-save-error")).Gap(8).Padding(12, 16).Margin(12, 0, 0).Shrink(0).Radius(v.visual.Theme.Radius).Background(v.visual.DangerSoft).Children(func() {
		ui.Text(c, v.draftSaveError).Role(ui.RoleStatus).FontSize(12).TextColor(v.visual.Danger)
		b := webButton(c, "重试保存草稿", false)
		b.Children(func() { webActionText(c, "重试保存草稿") })
		if b.Clicked() {
			v.flushNewTaskDraft()
			c.Invalidate()
		}
	})
}

// Preflight runs before any quit cleanup; a failed flush keeps the window,
// tray, notification worker and shortcuts alive so the user can retry.
func (h *nativeApp) flushMainDraft() bool {
	if h.view == nil || h.view.flushNewTaskDraft() {
		return true
	}
	if h.window != nil {
		h.window.Show()
		h.window.Focus()
		h.window.Invalidate()
	}
	return false
}

func (h *nativeApp) beforeQuit(e *mygo.QuitEvent) {
	if !e.DefaultPrevented() && !h.flushMainDraft() {
		e.PreventDefault()
	}
}

func onQuitCleanup(f func()) {
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if !e.DefaultPrevented() {
			f()
		}
	})
}
