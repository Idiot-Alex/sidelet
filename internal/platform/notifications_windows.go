//go:build windows

package platform

import (
	"errors"
	"sidelet/internal/reminder"
)

func StartNotifications(func(string, string)) {}
func StopNotifications()                      {}
func RequestNotificationPermission()          {}

type Notifications struct{}

func (Notifications) State() (reminder.State, error) {
	return reminder.State{Authorization: "unsupported"}, nil
}
func (Notifications) Deliver(string, string, string) error {
	return errors.New("system reminders are currently available on macOS")
}
func (Notifications) Remove([]string) error { return nil }
