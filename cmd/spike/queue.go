//go:build windows || darwin

package main

import "sync"

// Native callbacks can run on the UI thread while loop waits in InvokeSync.
// Never wait for queue capacity on that thread: synchronous window changes
// and bursts of screen/pointer notifications would deadlock both sides.
type messageQueue struct {
	mu      sync.Mutex
	pending []message
	head    int
	wake    chan struct{}
}

func newMessageQueue() *messageQueue {
	return &messageQueue{wake: make(chan struct{}, 1)}
}

func (q *messageQueue) post(m message, done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}
	q.mu.Lock()
	q.pending = append(q.pending, m)
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *messageQueue) next(done <-chan struct{}) (message, bool) {
	for {
		select {
		case <-done:
			return message{}, false
		default:
		}
		q.mu.Lock()
		if q.head < len(q.pending) {
			m := q.pending[q.head]
			q.pending[q.head] = message{} // Release window and event references.
			q.head++
			if q.head == len(q.pending) {
				q.pending = nil
				q.head = 0
			} else if q.head >= 256 && q.head >= len(q.pending)/2 {
				// Bound consumed backing storage even under continuous input.
				q.pending = append([]message(nil), q.pending[q.head:]...)
				q.head = 0
			}
			q.mu.Unlock()
			return m, true
		}
		q.mu.Unlock()
		select {
		case <-q.wake:
		case <-done:
			return message{}, false
		}
	}
}
