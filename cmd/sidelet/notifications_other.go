//go:build !darwin && !windows

package main

import (
	"errors"
	"sidelet/internal/reminder"
)

type unsupportedNotifications struct{}

func newNotificationBackend() notificationBackend { return unsupportedNotifications{} }
func (unsupportedNotifications) State() (reminder.State, error) {
	return reminder.State{Authorization: "unsupported"}, nil
}
func (unsupportedNotifications) Deliver(string, string, string) error {
	return errors.New("system reminders are not supported on this platform")
}
func (unsupportedNotifications) Remove([]string) error         { return nil }
func (unsupportedNotifications) RequestPermission(done func()) { done() }
func (unsupportedNotifications) Watch(func()) func()           { return func() {} }
func openNotificationSettings()                                {}
