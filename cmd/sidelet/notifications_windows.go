//go:build windows

package main

import "github.com/egoist/mygo"

// MyGo uses Shell_NotifyIconW, not WinRT toast history. Only a successful
// submission is acknowledged; Windows decides whether a banner is shown.
func newNotificationBackend() notificationBackend {
	return newWindowsNotificationSession(mygo.NotificationsSupported,
		func(id, title, body string) error {
			return mygo.NewNotification(mygo.NotificationOptions{
				ID: id, Title: title, Body: body, Group: "Sidelet",
			}).Show()
		}, func(id string) {
			mygo.NewNotification(mygo.NotificationOptions{ID: id}).Close()
		})
}

func (*windowsNotificationSession) RequestPermission(done func()) {
	// Notification-area notifications do not expose a per-app permission prompt.
	done()
}
func (*windowsNotificationSession) Watch(func()) func() { return func() {} }
func openNotificationSettings() {
	_ = mygo.Shell.OpenExternal("ms-settings:notifications")
}
