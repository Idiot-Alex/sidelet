//go:build !darwin && !windows

package main

import "errors"

type unsupportedQuickFocus struct{}

func captureQuickForeground() quickFocusToken { return unsupportedQuickFocus{} }
func quickForegroundIsOurs() bool             { return false }
func (unsupportedQuickFocus) restore() bool   { return false }
func (unsupportedQuickFocus) release()        {}
func (unsupportedQuickFocus) pid() int        { return 0 }
func registerNativeQuickAddShortcut(func()) (func(), error) {
	return nil, errors.New("global shortcut is unavailable on this prototype platform")
}
func registerNativeQuickAddDiagnosticShortcut(func()) (func(), error) {
	return nil, errors.New("diagnostic key is available only on macOS")
}
