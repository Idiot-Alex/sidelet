//go:build windows || darwin

package main

import (
	"sync"
	"testing"
	"time"
)

// Native callbacks run on the same UI thread that the controller waits for.
// Posting a burst must finish even while that thread cannot drain the queue.
func TestNativeBurstPostDoesNotWaitForMainThread(t *testing.T) {
	c := &controller{queue: newMessageQueue(), done: make(chan struct{})}
	defer close(c.done)
	posted := make(chan struct{})
	go func() {
		defer close(posted)
		for i := 0; i < 512; i++ {
			c.post(message{Type: "displays", Revision: uint64(i)})
		}
	}()
	select {
	case <-posted:
	case <-time.After(time.Second):
		t.Fatal("native callback blocked posting a burst while the UI thread could not drain messages")
	}
	for i := 0; i < 512; i++ {
		m, ok := c.queue.next(c.done)
		if !ok || m.Type != "displays" || m.Revision != uint64(i) {
			t.Fatalf("event %d was lost or reordered: %+v", i, m)
		}
	}
	if c.queue.pending != nil || c.queue.head != 0 {
		t.Fatal("drained queue retained event storage")
	}
}

func TestMessageQueueConcurrentProducers(t *testing.T) {
	q := newMessageQueue()
	done := make(chan struct{})
	defer close(done)
	var producers sync.WaitGroup
	for producer := 0; producer < 8; producer++ {
		producers.Add(1)
		go func(id int) {
			defer producers.Done()
			for i := 0; i < 512; i++ {
				q.post(message{Type: "pointer", X: float64(id), Revision: uint64(i)}, done)
			}
		}(producer)
	}
	finished := make(chan struct{})
	go func() { producers.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("concurrent producers waited for a consumer")
	}
	var nextRevision [8]uint64
	for i := 0; i < 8*512; i++ {
		m, ok := q.next(done)
		id := int(m.X)
		if !ok || id < 0 || id >= len(nextRevision) || m.Revision != nextRevision[id] {
			t.Fatalf("lost, duplicated, or reordered event: %+v", m)
		}
		nextRevision[id]++
	}
}

func TestMessageQueueWakeAndShutdown(t *testing.T) {
	q := newMessageQueue()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	finished := make(chan message, 1)
	go func() { m, _ := q.next(done); finished <- m }()
	q.post(message{Type: "keyboard", Source: "global-shortcut"}, done)
	select {
	case m := <-finished:
		if m.Type != "keyboard" || m.Source != "global-shortcut" {
			t.Fatalf("wrong event: %+v", m)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer missed a wakeup")
	}
	closed := make(chan struct{})
	close(closed)
	q.post(message{Type: "quit"}, closed)
	if _, ok := q.next(closed); ok {
		t.Fatal("shutdown delivered an event")
	}
	if q.pending != nil {
		t.Fatal("post after shutdown retained an event")
	}
	stopped := make(chan bool, 1)
	stop := make(chan struct{})
	go func() { _, ok := q.next(stop); stopped <- ok }()
	close(stop)
	select {
	case ok := <-stopped:
		if ok {
			t.Fatal("waiting consumer received an event on shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wake an idle consumer")
	}
}
