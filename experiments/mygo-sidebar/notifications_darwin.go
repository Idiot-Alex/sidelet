//go:build darwin

package main

import (
	"errors"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"sidelet/internal/reminder"
)

type macNotifications struct{}

func newNotificationBackend() notificationBackend { return macNotifications{} }
func notificationCenter() objc.ID {
	return send(objc.ID(objc.GetClass("UNUserNotificationCenter")), "currentNotificationCenter")
}
func nsString(text string) objc.ID {
	return send(objc.ID(objc.GetClass("NSString")), "stringWithUTF8String:", text)
}
func notificationIdentifiers(array objc.ID, delivered bool) []string {
	ids := []string{}
	count := int(send(array, "count"))
	for i := 0; i < count; i++ {
		item := send(array, "objectAtIndex:", uint(i))
		if delivered {
			item = send(item, "request")
		}
		id := send(item, "identifier")
		ids = append(ids, objc.Send[string](id, objc.RegisterName("UTF8String")))
	}
	return ids
}

// Only query this packaged app's centre. The asynchronous OS queries wait on
// the worker, never on AppKit's main thread, and do not request permission.
func (macNotifications) State() (reminder.State, error) {
	if !mygo.NotificationsSupported() {
		return reminder.State{Authorization: "unsupported"}, nil
	}
	type part struct {
		kind          string
		authorization string
		ids           []string
	}
	ch := make(chan part, 3)
	mygo.RunOnMain(func() {
		pool := send(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
		defer send(pool, "drain")
		center := notificationCenter()
		settings := objc.NewBlock(func(_ objc.Block, value objc.ID) {
			authorization := "notDetermined"
			switch int(send(value, "authorizationStatus")) {
			case 1:
				authorization = "denied"
			case 2, 3, 4:
				authorization = "authorized"
			}
			ch <- part{kind: "authorization", authorization: authorization}
		})
		send(center, "getNotificationSettingsWithCompletionHandler:", settings)
		settings.Release()
		pending := objc.NewBlock(func(_ objc.Block, value objc.ID) {
			ch <- part{kind: "pending", ids: notificationIdentifiers(value, false)}
		})
		send(center, "getPendingNotificationRequestsWithCompletionHandler:", pending)
		pending.Release()
		delivered := objc.NewBlock(func(_ objc.Block, value objc.ID) {
			ch <- part{kind: "delivered", ids: notificationIdentifiers(value, true)}
		})
		send(center, "getDeliveredNotificationsWithCompletionHandler:", delivered)
		delivered.Release()
	})
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	state := reminder.State{}
	for range 3 {
		select {
		case p := <-ch:
			switch p.kind {
			case "authorization":
				state.Authorization = p.authorization
			case "pending":
				state.Pending = p.ids
			case "delivered":
				state.Delivered = p.ids
			}
		case <-timer.C:
			return state, errors.New("notification status timed out")
		}
	}
	return state, nil
}
func (macNotifications) Deliver(id, title, body string) error {
	return mygo.NewNotification(mygo.NotificationOptions{ID: id, Title: title, Body: body, Group: "Sidelet MyGo Lab"}).Show()
}
func (macNotifications) Remove(ids []string) error {
	for _, id := range ids {
		mygo.NewNotification(mygo.NotificationOptions{ID: id}).Close()
	}
	return nil
}
func (macNotifications) RequestPermission(done func()) {
	if !mygo.NotificationsSupported() {
		done()
		return
	}
	mygo.RunOnMain(func() {
		pool := send(objc.ID(objc.GetClass("NSAutoreleasePool")), "new")
		defer send(pool, "drain")
		block := objc.NewBlock(func(_ objc.Block, _ bool, _ objc.ID) { mygo.RunOnMain(done) })
		send(notificationCenter(), "requestAuthorizationWithOptions:completionHandler:", uint(1<<1|1<<2), block)
		block.Release()
	})
}
func (macNotifications) Watch(refresh func()) func() {
	workspaceCenter := send(send(objc.ID(objc.GetClass("NSWorkspace")), "sharedWorkspace"), "notificationCenter")
	clockCenter := send(objc.ID(objc.GetClass("NSNotificationCenter")), "defaultCenter")
	queue := send(objc.ID(objc.GetClass("NSOperationQueue")), "mainQueue")
	block := objc.NewBlock(func(_ objc.Block, _ objc.ID) { refresh() })
	wake := send(workspaceCenter, "addObserverForName:object:queue:usingBlock:", nsString("NSWorkspaceDidWakeNotification"), objc.ID(0), queue, block)
	clock := send(clockCenter, "addObserverForName:object:queue:usingBlock:", nsString("NSSystemClockDidChangeNotification"), objc.ID(0), queue, block)
	block.Release()
	send(wake, "retain")
	send(clock, "retain")
	return func() {
		send(workspaceCenter, "removeObserver:", wake)
		send(clockCenter, "removeObserver:", clock)
		send(wake, "release")
		send(clock, "release")
	}
}
func openNotificationSettings() {
	_ = mygo.Shell.OpenExternal("x-apple.systempreferences:com.apple.Notifications-Settings.extension")
}
