//go:build darwin

package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/egoist/mygo"
)

// Diagnostic-only request from the separate owned background/input probe.
// This does not synthesize a key, and is never reported as a global key test.
func watchQuickAddRequests(open func()) func() {
	requests := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(requests, syscall.SIGUSR2)
	go func() {
		for {
			select {
			case <-requests:
				mygo.RunOnMain(open)
			case <-done:
				return
			}
		}
	}()
	return func() { signal.Stop(requests); close(done) }
}
func requestNativeQuickAdd(pid int) error {
	if pid <= 0 || pid == os.Getpid() {
		return errors.New("invalid diagnostic target")
	}
	return syscall.Kill(pid, syscall.SIGUSR2)
}
