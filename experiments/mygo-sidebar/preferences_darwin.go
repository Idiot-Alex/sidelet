//go:build darwin

package main

import (
	"errors"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func dockPreferenceAvailable() bool { return true }

func applyDockPreference(show bool) error {
	policy := mygo.ActivationPolicyAccessory
	want := uintptr(1)
	if show {
		policy, want = mygo.ActivationPolicyRegular, 0
	}
	mygo.App.SetActivationPolicy(policy)
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	if uintptr(app.Send(objc.RegisterName("activationPolicy"))) != want {
		return errors.New("无法更改 Dock 显示，请重试。")
	}
	return nil
}
