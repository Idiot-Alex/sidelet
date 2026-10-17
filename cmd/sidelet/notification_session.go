package main

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"

	"sidelet/internal/reminder"
)

// windowsNotificationSession tracks accepted requests, not Windows history.
// Keeping this core platform-independent lets tests exercise submission failure,
// receipt recovery and profile-scoped cleanup without a Windows desktop.
// SQLite receipts remain the source of truth across application restarts.
type windowsNotificationSession struct {
	supported func() bool
	show      func(id, title, body string) error
	close     func(id string)
	mu        sync.Mutex
	accepted  map[string]bool
}

func newWindowsNotificationSession(supported func() bool, show func(string, string, string) error, close func(string)) *windowsNotificationSession {
	return &windowsNotificationSession{supported: supported, show: show, close: close, accepted: map[string]bool{}}
}

func (n *windowsNotificationSession) State() (reminder.State, error) {
	if !n.supported() {
		return reminder.State{Authorization: "unsupported"}, nil
	}
	n.mu.Lock()
	ids := make([]string, 0, len(n.accepted))
	for id := range n.accepted {
		ids = append(ids, id)
	}
	n.mu.Unlock()
	sort.Strings(ids)
	return reminder.State{Authorization: "systemManaged", Accepted: ids}, nil
}

func (n *windowsNotificationSession) Deliver(id, title, body string) error {
	if id == "" {
		return errors.New("notification identifier is empty")
	}
	if !n.supported() {
		return errors.New("Windows notifications are unavailable")
	}
	n.mu.Lock()
	accepted := n.accepted[id]
	n.mu.Unlock()
	if accepted {
		return nil
	}
	// NOTIFYICONDATAW reserves the last UTF-16 unit for NUL. Do not split an
	// emoji's surrogate pair or let an embedded NUL truncate subsequent text.
	if err := n.show(id, windowsNotificationText(title, 63), windowsNotificationText(body, 255)); err != nil {
		return err
	}
	n.mu.Lock()
	n.accepted[id] = true
	n.mu.Unlock()
	return nil
}

func (n *windowsNotificationSession) Remove(ids []string) error {
	for _, id := range ids {
		n.mu.Lock()
		owned := n.accepted[id]
		delete(n.accepted, id)
		n.mu.Unlock()
		if owned {
			n.close(id)
		}
	}
	return nil
}

func windowsNotificationText(text string, capacity int) string {
	var b strings.Builder
	for _, r := range text {
		if r == 0 {
			continue
		}
		width := utf16.RuneLen(r)
		if width > capacity {
			break
		}
		b.WriteRune(r)
		capacity -= width
	}
	return b.String()
}
