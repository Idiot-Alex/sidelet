//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework UserNotifications
#include <stdlib.h>
#include "notifications_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"sidelet/internal/reminder"
	"sync"
	"unsafe"
)

var notificationCallback struct {
	sync.RWMutex
	callback func(string, string)
}

func StartNotifications(callback func(string, string)) {
	notificationCallback.Lock()
	notificationCallback.callback = callback
	notificationCallback.Unlock()
	C.SLNotificationStart()
}
func StopNotifications()             { C.SLNotificationStop() }
func RequestNotificationPermission() { C.SLNotificationRequestPermission() }

//export sideletNotificationEvent
func sideletNotificationEvent(kind, identifier *C.char) {
	notificationCallback.RLock()
	callback := notificationCallback.callback
	notificationCallback.RUnlock()
	if callback != nil {
		callback(C.GoString(kind), C.GoString(identifier))
	}
}
func notificationResult(raw *C.char, target any) error {
	if raw == nil {
		return errors.New("notification service unavailable")
	}
	defer C.free(unsafe.Pointer(raw))
	data := []byte(C.GoString(raw))
	var result struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	if target != nil {
		return json.Unmarshal(data, target)
	}
	return nil
}

type Notifications struct{}

func (Notifications) State() (reminder.State, error) {
	var state reminder.State
	err := notificationResult(C.SLNotificationState(), &state)
	return state, err
}
func (Notifications) Deliver(id, title, body string) error {
	i, t, b := C.CString(id), C.CString(title), C.CString(body)
	defer C.free(unsafe.Pointer(i))
	defer C.free(unsafe.Pointer(t))
	defer C.free(unsafe.Pointer(b))
	return notificationResult(C.SLNotificationDeliver(i, t, b), nil)
}
func (Notifications) Remove(ids []string) error {
	data, _ := json.Marshal(ids)
	raw := C.CString(string(data))
	defer C.free(unsafe.Pointer(raw))
	return notificationResult(C.SLNotificationRemove(raw), nil)
}
