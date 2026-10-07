package main

import (
	"strings"
	"sync"
	"testing"
)

func testService(m *model) *TasksService {
	var mu sync.Mutex
	return &TasksService{m: m, run: func(fn func()) { mu.Lock(); defer mu.Unlock(); fn() }}
}

func TestHybridSharesNativeTasksAndRejectsStaleDrafts(t *testing.T) {
	m := newModel()
	s := testService(m)
	var events []string
	s.changed = func(event string) { events = append(events, event) }
	before := s.List()
	added, err := s.Add("  主窗口新任务  ", 2)
	if err != nil || m.Tasks[3].Title != "主窗口新任务" || added.Revision != 1 || added.Tasks[3].ID != 4 {
		t.Fatalf("add: %+v, %v", added, err)
	}
	// List returns a copy, never a slice alias used by the native view.
	before.Tasks[0].Title = "不可写入模型"
	if m.Tasks[0].Title == before.Tasks[0].Title {
		t.Fatal("snapshot aliases model")
	}
	m.open(0)
	m.edit()
	m.Draft = "原生草稿"
	web, err := s.Update(1, "主窗口保存", 3, 1)
	if err != nil || web.Tasks[0].Title != "主窗口保存" {
		t.Fatal(err)
	}
	if m.save() || m.Tasks[0].Title != "主窗口保存" || m.EditError == "" {
		t.Fatal("stale native draft overwrote Web mutation")
	}
	m.cancel()
	m.edit()
	m.Draft = "原生保存"
	if !m.save() {
		t.Fatal("native save after re-edit failed")
	}
	if _, err = s.Update(1, "旧 Web 草稿", 0, web.Tasks[0].Version); err == nil || m.Tasks[0].Title != "原生保存" {
		t.Fatal("stale Web draft overwrote native save")
	}
	if strings.Join(events, ",") != "web-add,web-update" {
		t.Fatal(events)
	}
}

func TestHybridCompletionKeepsStableIDsAndCompactsSidebar(t *testing.T) {
	m := newModel()
	m.WorkHeight = 1000
	s := testService(m)
	m.open(0)
	m.edit()
	if _, err := s.SetDone(1, true, 1); err != nil {
		t.Fatal(err)
	}
	if m.Editing || m.Opened != -1 || m.stackHeight() != 116 || m.rowFor(1) != 0 {
		t.Fatalf("completion: %+v", m)
	}
	regions := stackInputRegions(m)
	if regions[0].task != 1 || regions[0].rect.Y != 8 {
		t.Fatal("sidebar did not compact")
	}
	if _, err := s.Add("新增后 ID 不变", 0); err != nil {
		t.Fatal(err)
	}
	if s.List().Tasks[3].ID != 4 || m.Tasks[1].Title != "确认设计稿" {
		t.Fatal("IDs changed")
	}
	if _, err := s.SetDone(1, false, m.Tasks[0].Version); err != nil {
		t.Fatal(err)
	}
	if m.rowFor(1) != 1 || m.stackHeight() != 216 {
		t.Fatal("restored task was not rendered")
	}
}

func TestHybridInvalidInputAndOpenPreserveState(t *testing.T) {
	m := newModel()
	s := testService(m)
	for _, tc := range []struct {
		title    string
		priority int
	}{{" ", 0}, {strings.Repeat("中", 121), 0}, {"标题", 1}} {
		if _, err := s.Add(tc.title, tc.priority); err == nil {
			t.Fatal("invalid task accepted")
		}
	}
	if _, err := s.Update(0, "标题", 0, 1); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if s.revision != 0 || len(m.Tasks) != 3 {
		t.Fatal("failure mutated model")
	}
	m.open(0)
	m.edit()
	m.Draft = "保留草稿"
	if err := s.Open(2); err == nil || m.Draft != "保留草稿" {
		t.Fatal("open discarded draft")
	}
	m.cancel()
	opened := -1
	s.open = func(index int) { opened = index }
	if err := s.Open(2); err != nil || opened != 1 || m.Opened != 1 {
		t.Fatal("open did not use shared task")
	}
}

func TestHybridBridgeSerializesConcurrentCalls(t *testing.T) {
	m := newModel()
	s := testService(m)
	var wg sync.WaitGroup
	for n := 0; n < 24; n++ {
		wg.Go(func() {
			for i := 0; i < 12; i++ {
				tasks := s.List()
				_, _ = s.Update(1, "并发更新", 0, tasks.Tasks[0].Version)
			}
		})
	}
	wg.Wait()
	final := s.List()
	if final.Tasks[0].Version != final.Revision+1 || final.Revision == 0 {
		t.Fatal("updates were not serialized")
	}
}

func TestHybridCapacityAllowsAddingAfterCompletion(t *testing.T) {
	m := newModel()
	s := testService(m)
	for m.activeCount() < 8 {
		if _, err := s.Add("合成任务", 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Add("超出侧栏", 0); err == nil {
		t.Fatal("capacity limit missing")
	}
	if _, err := s.SetDone(1, true, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("完成后添加", 0); err != nil {
		t.Fatal(err)
	}
}
