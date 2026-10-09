//go:build !darwin

package main

import "errors"

type unsupportedLogin struct{}

func newLoginBackend() loginBackend              { return unsupportedLogin{} }
func (unsupportedLogin) Status() (string, error) { return "unsupported", nil }
func (unsupportedLogin) Set(bool) error          { return errors.New("登录启动目前仅支持 macOS。") }
func (unsupportedLogin) OpenSettings() error {
	return errors.New("登录启动目前仅支持 macOS。")
}
