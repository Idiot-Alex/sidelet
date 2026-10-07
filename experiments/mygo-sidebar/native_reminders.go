package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"sidelet/internal/reminder"
	"sidelet/internal/todo"
)

var errReminderSuperseded = errors.New("reminder superseded")

type notificationBackend interface {
	reminder.System
	RequestPermission(func())
	Watch(func()) func()
}

type reminderJob struct {
	epoch uint64
	now   time.Time
	rows  []todo.Reminder
}
type receiptRepo struct {
	rows     []todo.Reminder
	accepted map[int64]int64
}

func (r *receiptRepo) Reminders() ([]todo.Reminder, error) { return r.rows, nil }
func (r *receiptRepo) MarkReminderSent(id int64, now time.Time) error {
	r.accepted[id] = now.UnixMilli()
	return nil
}

// All generations and task metadata belong to the UI thread. A single worker
// receives immutable copies and uses the same Sync as the production store.
// Before delivery/removal it validates against the live generations; receipts
// arriving after an edit/delete never consume a replacement reminder.
type nativeReminders struct {
	s          *TasksService
	backend    notificationBackend
	prefix     string
	rows       map[int]todo.Reminder
	generation int64
	epoch      uint64
	jobs       chan reminderJob
	stop       chan struct{}
	stopped    atomic.Bool
	unwatch    func()
}

func newNativeReminders(s *TasksService, backend notificationBackend, prefix string) *nativeReminders {
	return &nativeReminders{s: s, backend: backend, prefix: prefix, rows: map[int]todo.Reminder{}, jobs: make(chan reminderJob, 1), stop: make(chan struct{})}
}
func (n *nativeReminders) reconcile() {
	for i := range n.s.m.Tasks {
		t := &n.s.m.Tasks[i]
		id := i + 1
		if t.Deleted || !t.Remind || t.DueAt == 0 {
			delete(n.rows, id)
			t.ReminderSentAt = 0
			continue
		}
		at := todo.EffectiveReminderAt(t.DueAt, t.SnoozedUntil)
		r, ok := n.rows[id]
		if !ok || r.At != at {
			n.generation++
			r = todo.Reminder{ID: n.generation, TodoID: int64(id), At: at}
			t.ReminderSentAt = 0
		}
		r.Title, r.Completed, r.SentAt = t.Title, t.Done, t.ReminderSentAt
		n.rows[id] = r
	}
}
func (n *nativeReminders) request() {
	if n.stopped.Load() {
		return
	}
	n.reconcile()
	n.epoch++
	job := reminderJob{epoch: n.epoch, now: n.s.m.now(), rows: make([]todo.Reminder, 0, len(n.rows))}
	for _, r := range n.rows {
		job.rows = append(job.rows, r)
	}
	sort.Slice(job.rows, func(i, j int) bool { return job.rows[i].ID < job.rows[j].ID })
	select {
	case <-n.jobs:
	default:
	}
	n.jobs <- job
}
func (n *nativeReminders) worker() {
	for {
		select {
		case <-n.stop:
			return
		case job := <-n.jobs:
			repo := &receiptRepo{rows: job.rows, accepted: map[int64]int64{}}
			result, err := reminder.Sync(repo, liveNotificationSystem{n}, n.prefix, job.now)
			if n.stopped.Load() {
				return
			}
			n.s.run(func() {
				if !n.stopped.Load() {
					n.accept(job, repo.accepted, result, err)
				}
			})
		}
	}
}
func (n *nativeReminders) accept(job reminderJob, receipts map[int64]int64, result reminder.Result, err error) {
	for id, r := range n.rows {
		if at, ok := receipts[r.ID]; ok {
			t := &n.s.m.Tasks[id-1]
			t.ReminderSentAt = at
			r.SentAt = at
			n.rows[id] = r
		}
	}
	if job.epoch == n.epoch {
		n.s.notificationAuthorization = result.Authorization
		n.s.notificationDelivered = result.SystemDelivered
		n.s.notificationStatus = notificationMessage(result.Authorization, err)
		n.s.nextReminder = result.Next
		if err != nil && !errors.Is(err, errReminderSuperseded) {
			n.s.nextReminder = n.s.m.now().Add(time.Minute)
		}
	}
	n.s.scheduleExpiry()
	if n.s.changed != nil {
		n.s.changed("native-main-reminder-status")
	}
}
func notificationMessage(authorization string, err error) string {
	if err != nil && !errors.Is(err, errReminderSuperseded) {
		return "系统提醒暂时不可用，将稍后重试。"
	}
	switch authorization {
	case "denied":
		return "系统通知未获允许。请在 macOS 系统设置的通知中允许 Sidelet MyGo Lab，然后检查权限。"
	case "notDetermined":
		return "开启系统通知后，勾选“提醒我”的任务才会发送通知。"
	case "unsupported":
		return "当前平台暂不支持系统通知，桌面到期状态仍可使用。"
	default:
		return ""
	}
}
func (n *nativeReminders) active(identifier string) (todo.Reminder, bool) {
	for _, r := range n.rows {
		if identifier == fmt.Sprintf("%s%d", n.prefix, r.ID) {
			return r, !r.Completed
		}
	}
	return todo.Reminder{}, false
}
func (n *nativeReminders) permission() {
	n.backend.RequestPermission(func() {
		if !n.stopped.Load() {
			n.request()
		}
	})
}
func (n *nativeReminders) close() {
	if n.stopped.Swap(true) {
		return
	}
	close(n.stop)
	if n.unwatch != nil {
		n.unwatch()
	}
	// This experiment owns only its current run's namespace. Never clear other
	// apps or previous profiles through ClearNotifications.
	ids := make([]string, 0, n.generation)
	for id := int64(1); id <= n.generation; id++ {
		ids = append(ids, fmt.Sprintf("%s%d", n.prefix, id))
	}
	_ = n.backend.Remove(ids)
}

type liveNotificationSystem struct{ n *nativeReminders }

func (x liveNotificationSystem) State() (reminder.State, error) { return x.n.backend.State() }
func (x liveNotificationSystem) Deliver(id, title, body string) error {
	if x.n.stopped.Load() {
		return errReminderSuperseded
	}
	alive := false
	x.n.s.run(func() { r, ok := x.n.active(id); alive = ok; title = r.Title })
	if !alive || x.n.stopped.Load() {
		return errReminderSuperseded
	}
	return x.n.backend.Deliver(id, title, body)
}
func (x liveNotificationSystem) Remove(ids []string) error {
	if x.n.stopped.Load() {
		return errReminderSuperseded
	}
	stale := []string{}
	x.n.s.run(func() {
		for _, id := range ids {
			if _, alive := x.n.active(id); !alive {
				stale = append(stale, id)
			}
		}
	})
	if len(stale) == 0 {
		return nil
	}
	return x.n.backend.Remove(stale)
}
func (s *TasksService) enableNotifications(h *hybridApp) {
	mygo.App.WhenReady(func() {
		n := newNativeReminders(s, newNotificationBackend(), fmt.Sprintf("sidelet-mygo.%d.", time.Now().UnixNano()))
		s.notices = n
		go n.worker()
		n.unwatch = n.backend.Watch(func() {
			if !n.stopped.Load() {
				s.m.cleanup()
				n.request()
				s.scheduleExpiry()
				if s.changed != nil {
					s.changed("native-main-clock-refresh")
				}
			}
		})
		n.request()
		mygo.App.OnActivate(func(bool) {
			if !n.stopped.Load() {
				n.request()
			}
		})
		mygo.App.OnNotificationClick(func(id string) {
			if strings.HasPrefix(id, n.prefix) {
				h.show()
			}
		})
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { n.close() })
	})
}
