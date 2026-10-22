//go:build !darwin && !windows

package main

import "errors"

type unsupportedLogin struct{}

func newLoginBackend() loginBackend              { return unsupportedLogin{} }
func (unsupportedLogin) Status() (string, error) { return "unsupported", nil }
func (unsupportedLogin) Set(bool) error          { return errors.New("当前平台暂不支持登录启动。") }
func (unsupportedLogin) OpenSettings() error {
	return errors.New("当前平台暂不支持登录启动。")
}
