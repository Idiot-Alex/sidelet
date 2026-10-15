//go:build !darwin

package main

import (
	"errors"
	"sidelet/internal/reminder"
)

// Windows remains compile-only; keep its runtime capability claim honest.
type unsupportedNotifications struct{}

func newNotificationBackend() notificationBackend { return unsupportedNotifications{} }
func (unsupportedNotifications) State() (reminder.State, error) {
	return reminder.State{Authorization: "unsupported"}, nil
}
func (unsupportedNotifications) Deliver(string, string, string) error {
	return errors.New("system reminders are currently available on macOS")
}
func (unsupportedNotifications) Remove([]string) error         { return nil }
func (unsupportedNotifications) RequestPermission(done func()) { done() }
func (unsupportedNotifications) Watch(func()) func()           { return func() {} }
func openNotificationSettings()                                {}
