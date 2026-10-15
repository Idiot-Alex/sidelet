//go:build !darwin

package main

import "errors"

func dockPreferenceAvailable() bool { return false }
func applyDockPreference(bool) error {
	return errors.New("当前平台暂不支持 Dock 显示设置。")
}
