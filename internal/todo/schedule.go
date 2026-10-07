package todo

import (
	"errors"
	"time"
)

// Reminder generations follow the later of the deadline and snooze time.
type Reminder struct {
	ID, TodoID int64
	Title      string
	At, SentAt int64 // Accepted by the OS; this does not prove a banner appeared.
	Completed  bool
}

func ValidateDeadline(dueAt int64, remind bool) error {
	if dueAt < 0 || dueAt > 253402300799999 {
		return errors.New("截止时间无效。")
	}
	if remind && dueAt == 0 {
		return errors.New("请先设置截止时间，再开启提醒。")
	}
	return nil
}
func EffectiveReminderAt(dueAt, snoozedUntil int64) int64 { return max(dueAt, snoozedUntil) }
func TemporaryExpired(temporary, completed bool, completedAt int64, now time.Time) bool {
	return temporary && completed && completedAt > 0 && now.UnixMilli() > completedAt+5000
}
