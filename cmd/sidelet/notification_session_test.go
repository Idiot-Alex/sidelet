package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"sidelet/internal/reminder"
	"sidelet/internal/todo"
)

type sessionReceiptRepo struct {
	rows     []todo.Reminder
	failMark bool
}

func (r *sessionReceiptRepo) Reminders() ([]todo.Reminder, error) { return r.rows, nil }
func (r *sessionReceiptRepo) MarkReminderSent(id int64, at time.Time) error {
	if r.failMark {
		return errors.New("disk unavailable")
	}
	for i := range r.rows {
		if r.rows[i].ID == id {
			r.rows[i].SentAt = at.UnixMilli()
		}
	}
	return nil
}

func TestWindowsNotificationSubmissionAndReceiptRecovery(t *testing.T) {
	now := time.Unix(1000, 0)
	r := &sessionReceiptRepo{rows: []todo.Reminder{{ID: 1, At: 1, Title: "中文提醒 🔔"}}}
	calls, fail := 0, true
	n := newWindowsNotificationSession(func() bool { return true }, func(id, title, body string) error {
		calls++
		if id != "own.1" || title != r.rows[0].Title || body == "" {
			t.Fatal("lost notification content or identifier")
		}
		if fail {
			return errors.New("Windows rejected submission")
		}
		return nil
	}, func(string) {})
	if _, err := reminder.Sync(r, n, "own.", now); err == nil || r.rows[0].SentAt != 0 {
		t.Fatal("failed submission consumed reminder")
	}
	state, _ := n.State()
	if state.Authorization != "systemManaged" || len(state.Accepted) != 0 || len(state.Delivered) != 0 {
		t.Fatal("submission was presented as permission or visible history")
	}
	fail, r.failMark = false, true
	if _, err := reminder.Sync(r, n, "own.", now); err == nil || calls != 2 {
		t.Fatal("receipt write failure not surfaced")
	}
	r.failMark = false
	result, err := reminder.Sync(r, n, "own.", now)
	if err != nil || calls != 2 || r.rows[0].SentAt == 0 || result.SystemDelivered != 0 {
		t.Fatal("receipt recovery resent submission or reported visible notification", result, err)
	}
	// A new process has no history, but durable receipts still prevent replay.
	restarted := newWindowsNotificationSession(func() bool { return true }, func(string, string, string) error {
		t.Fatal("restarted application resent accepted reminder")
		return nil
	}, func(string) {})
	if _, err := reminder.Sync(r, restarted, "own.", now); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsNotificationCleanupOwnsOnlyAcceptedIdentifiers(t *testing.T) {
	var closed []string
	n := newWindowsNotificationSession(func() bool { return true }, func(string, string, string) error { return nil }, func(id string) { closed = append(closed, id) })
	for _, id := range []string{"own.1", "own.2", "other.1"} {
		if err := n.Deliver(id, "标题", "正文"); err != nil {
			t.Fatal(err)
		}
	}
	r := &sessionReceiptRepo{rows: []todo.Reminder{{ID: 1, At: 1, SentAt: 1}, {ID: 2, Completed: true}}}
	if _, err := reminder.Sync(r, n, "own.", time.Now()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(closed, []string{"own.2"}) {
		t.Fatal("cleanup removed a live reminder or another profile", closed)
	}
	_ = n.Remove([]string{"unknown.1", "own.2"})
	if len(closed) != 1 {
		t.Fatal("cleanup touched an unowned or already removed notification")
	}
	state, _ := n.State()
	if !reflect.DeepEqual(state.Accepted, []string{"other.1", "own.1"}) {
		t.Fatal("cleanup lost active identifiers", state)
	}
}

func TestWindowsNotificationUnsupportedAndDuplicateSubmission(t *testing.T) {
	supported, calls := false, 0
	n := newWindowsNotificationSession(func() bool { return supported }, func(string, string, string) error { calls++; return nil }, func(string) {})
	state, _ := n.State()
	if state.Authorization != "unsupported" || n.Deliver("id", "title", "body") == nil || calls != 0 {
		t.Fatal("unsupported backend attempted delivery")
	}
	supported = true
	if n.Deliver("", "title", "body") == nil {
		t.Fatal("empty identifier accepted")
	}
	for range 2 {
		if err := n.Deliver("id", "title", "body"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("same identifier submitted twice")
	}
}

func TestWindowsNotificationTextPreservesUnicodeAtNativeLimits(t *testing.T) {
	for _, tt := range []struct {
		text  string
		limit int
		want  string
	}{
		{"中文 🔔 & <提醒>", 63, "中文 🔔 & <提醒>"},
		{"前\x00后", 63, "前后"},
		{strings.Repeat("中", 62) + "🔔末", 63, strings.Repeat("中", 62)},
		{strings.Repeat("中", 61) + "🔔末", 63, strings.Repeat("中", 61) + "🔔"},
		{strings.Repeat("🔔", 128), 255, strings.Repeat("🔔", 127)},
	} {
		got := windowsNotificationText(tt.text, tt.limit)
		if got != tt.want || !utf8.ValidString(got) || len(utf16.Encode([]rune(got))) > tt.limit {
			t.Fatalf("UTF-16 limit %d produced invalid/truncated content: %q", tt.limit, got)
		}
	}
}
