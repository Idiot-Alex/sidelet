// Package reminder reconciles durable reminder generations with system delivery.
// The caller serializes Sync with task mutations. It never polls when idle.
package reminder

import (
	"fmt"
	"sidelet/internal/todo"
	"strings"
	"time"
)

type State struct {
	Authorization string   `json:"authorization"`
	Pending       []string `json:"pending"`
	Delivered     []string `json:"delivered"`
}
type System interface {
	State() (State, error)
	Deliver(id, title, body string) error
	Remove(ids []string) error
}
type Repository interface {
	Reminders() ([]todo.Reminder, error)
	MarkReminderSent(int64, time.Time) error
}
type Result struct {
	Authorization   string
	Next            time.Time
	Changed         bool
	SystemDelivered int
}

func Sync(repo Repository, system System, prefix string, now time.Time) (Result, error) {
	result := Result{}
	reminders, err := repo.Reminders()
	if err != nil {
		return result, err
	}
	state, err := system.State()
	if err != nil {
		return result, err
	}
	result.Authorization = state.Authorization
	for _, id := range state.Delivered {
		if strings.HasPrefix(id, prefix) {
			result.SystemDelivered++
		}
	}
	alive := map[string]bool{}
	seen := map[string]bool{}
	for _, r := range reminders {
		if !r.Completed {
			alive[fmt.Sprintf("%s%d", prefix, r.ID)] = true
		}
	}
	stale := []string{}
	for _, id := range append(state.Pending, state.Delivered...) {
		seen[id] = true
		if strings.HasPrefix(id, prefix) && !alive[id] {
			stale = append(stale, id)
		}
	}
	if len(stale) > 0 {
		if err = system.Remove(stale); err != nil {
			return result, err
		}
	}
	for _, r := range reminders {
		if r.Completed || r.SentAt != 0 {
			continue
		}
		at := time.UnixMilli(r.At)
		if at.After(now) {
			if result.Next.IsZero() || at.Before(result.Next) {
				result.Next = at
			}
			continue
		}
		id := fmt.Sprintf("%s%d", prefix, r.ID)
		// Recover a crash between OS acceptance and the SQLite acknowledgement.
		if !seen[id] {
			if state.Authorization != "authorized" {
				continue
			}
			body := "任务已到提醒时间"
			if now.Sub(at) > time.Minute {
				body = "错过的任务提醒 · " + at.Local().Format("01/02 15:04")
			}
			if err = system.Deliver(id, r.Title, body); err != nil {
				return result, err
			}
		}
		if err = repo.MarkReminderSent(r.ID, now); err != nil {
			return result, err
		}
		result.Changed = true
	}
	return result, nil
}
