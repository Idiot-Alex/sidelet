//go:build darwin

package main

import (
	"testing"
	"time"
)

func TestFullscreenProbeStopsOutsideFullscreen(t *testing.T) {
	c := &controller{queue: newMessageQueue(), done: make(chan struct{})}
	defer close(c.done)
	c.scheduleFullscreenCheck()
	if c.fullscreenTimer != nil {
		t.Fatal("normal idle installed a fullscreen timer")
	}
	c.fullscreen = true
	c.scheduleFullscreenCheck()
	if c.fullscreenTimer == nil {
		t.Fatal("known fullscreen session needs an exit check")
	}
	c.fullscreen = false
	c.scheduleFullscreenCheck()
	if c.fullscreenTimer != nil {
		t.Fatal("fullscreen exit left a timer installed")
	}
	select {
	case <-c.queue.wake:
		t.Fatal("cancelled fullscreen check posted a message")
	case <-time.After(1200 * time.Millisecond):
	}
}

func TestFullscreenProbeWaitsForHandlerRearm(t *testing.T) {
	c := &controller{queue: newMessageQueue(), done: make(chan struct{}), fullscreen: true}
	defer close(c.done)
	c.scheduleFullscreenCheck()
	defer c.fullscreenTimer.Stop()
	select {
	case <-c.queue.wake:
		m, _ := c.queue.next(c.done)
		if m.Type != "fullscreen-check" {
			t.Fatalf("unexpected message: %s", m.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fullscreen exit check was not posted")
	}
	select {
	case <-c.queue.wake:
		t.Fatal("timer recurred without the controller rearming it")
	case <-time.After(1200 * time.Millisecond):
	}
}
