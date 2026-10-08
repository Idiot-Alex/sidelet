package main

import (
	"errors"
	"sidelet/internal/todo"
	"strings"
	"time"
)

type taskSchedule struct {
	DueAt             int64
	Remind, Temporary bool
}

func (o taskSchedule) apply(t *task) error {
	if err := todo.ValidateDeadline(o.DueAt, o.Remind); err != nil {
		return err
	}
	if t.DueAt != o.DueAt || t.Remind != o.Remind {
		t.ReminderID, t.ReminderAt, t.ReminderSentAt = 0, 0, 0
	}
	t.DueAt, t.Remind, t.Temporary = o.DueAt, o.Remind, o.Temporary
	return nil
}
func (s *TasksService) SaveTask(id int, title, note string, priority int, pinned bool, dueAt int64, remind, temporary bool, version uint64) (taskSnapshot, error) {
	options := &taskSchedule{dueAt, remind, temporary}
	if id == 0 {
		return s.addWithSchedule(title, note, priority, pinned, options)
	}
	return s.update(id, title, &note, priority, version, options)
}
func dueLabel(t task, now time.Time) string {
	if t.DueAt == 0 || t.Done || t.SnoozedUntil > now.UnixMilli() {
		return ""
	}
	if t.DueAt <= now.UnixMilli() {
		return "已逾期"
	}
	if t.DueAt-now.UnixMilli() <= 30*time.Minute.Milliseconds() {
		return "即将到期"
	}
	return ""
}
func dueText(t task, now time.Time) string {
	text := time.UnixMilli(t.DueAt).In(now.Location()).Format("1/2 15:04")
	if label := dueLabel(t, now); label != "" {
		return label + " · " + text
	}
	return text
}

// Preserve sub-minute timestamps when only other fields were edited, as in
// TodoManager. Parse local wall time and reject invalid/nonexistent dates.
func parseDue(text string, original int64, originalText string, zone *time.Location) (int64, error) {
	if text == originalText {
		return original, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	for _, format := range []string{"2006-01-02T15:04", "2006-01-02 15:04", "2006/01/02 15:04"} {
		stamp, err := time.ParseInLocation(format, text, zone)
		if err == nil && stamp.Format(format) == text {
			at := stamp.UnixMilli()
			if err := todo.ValidateDeadline(at, false); err == nil {
				return at, nil
			}
		}
	}
	return 0, errors.New("请填写有效的截止日期和时间。")
}
func localDue(at int64, zone *time.Location) string {
	if at == 0 {
		return ""
	}
	return time.UnixMilli(at).In(zone).Format("2006-01-02T15:04")
}
