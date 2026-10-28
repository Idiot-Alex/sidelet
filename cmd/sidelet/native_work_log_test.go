package main

import (
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestWorkLogUsesLocalDatesAndTaskState(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*60*60)
	day := time.Date(2026, 12, 31, 0, 0, 0, 0, zone)
	tomorrow := day.AddDate(0, 0, 1)
	tasks := []task{
		{Title: "晚完成", Done: true, CompletedAt: day.Add(23 * time.Hour).UnixMilli()},
		{Title: "零点完成", Done: true, CompletedAt: day.UnixMilli()},
		{Title: "昨天完成", Done: true, CompletedAt: day.Add(-time.Millisecond).UnixMilli()},
		{Title: "明天完成", Done: true, CompletedAt: tomorrow.UnixMilli()},
		{Title: "未知完成时间", Done: true},
		{Title: "已删除", Done: true, Deleted: true, CompletedAt: day.UnixMilli()},
		{Title: "已撤销", CompletedAt: day.UnixMilli()},
		{Title: "明晚计划", DueAt: tomorrow.Add(18 * time.Hour).UnixMilli()},
		{Title: "明早计划", DueAt: tomorrow.UnixMilli(), Unpinned: true},
		{Title: "后天计划", DueAt: tomorrow.AddDate(0, 0, 1).UnixMilli()},
		{Title: "今日逾期", DueAt: day.UnixMilli()},
		{Title: "已提前完成", Done: true, DueAt: tomorrow.UnixMilli()},
		{Title: "已删计划", Deleted: true, DueAt: tomorrow.UnixMilli()},
	}
	log := generateWorkLog(tasks, day.Add(12*time.Hour))
	if len(log.done) != 2 || log.done[0].Title != "零点完成" || len(log.planned) != 2 || log.planned[0].Title != "明早计划" {
		t.Fatalf("incorrect dates, sorting or state: %+v", log)
	}
	text := log.text()
	if !strings.Contains(text, "2026年12月31日") || !strings.Contains(text, "明日计划 · 1月1日（2 项）") || !strings.Contains(text, "预计 18:00") {
		t.Fatal(text)
	}
}

func TestWorkLogPreviewCopyAndReturn(t *testing.T) {
	m, v, u := webUITester(t)
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.Local)
	m.clock = func() time.Time { return now }
	m.Tasks = []task{{Title: "写完说明", Done: true, CompletedAt: now.UnixMilli()}, {Title: "发布版本", DueAt: now.AddDate(0, 0, 1).UnixMilli()}}
	if err := u.Click("工作日志"); err != nil {
		t.Fatal(err)
	}
	if !v.workLogOpen || !u.HasText("工作日志预览") && !u.HasText(generateWorkLog(m.Tasks, now).text()) {
		t.Fatal("missing log preview")
	}
	if path := os.Getenv("SIDELET_WORK_LOG_VISUAL"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, u.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if err := u.Click("复制日志"); err != nil {
		t.Fatal(err)
	}
	if u.Clipboard() != generateWorkLog(m.Tasks, now).text() {
		t.Fatal("clipboard differs from preview", u.Clipboard())
	}
	if !u.HasText("已复制，可直接粘贴分享") {
		t.Fatal("missing copy feedback")
	}
	// The log is regenerated from current data, not cached on opening.
	m.Tasks[0].Done = false
	u.Frame()
	if err := u.Click("复制日志"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(u.Clipboard(), "完成于") {
		t.Fatal("undo left a stale completion in the log")
	}
	u.Key(0, ui.KeyEscape)
	if v.workLogOpen || !u.HasText("我的任务") {
		t.Fatal("Escape did not return to tasks")
	}
}

func TestWorkLogEmptyAndDSTBoundary(t *testing.T) {
	if text := generateWorkLog(nil, time.Now()).text(); !strings.Contains(text, "暂无已完成任务") || !strings.Contains(text, "暂无预计明天完成的任务") {
		t.Fatal(text)
	}
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone database unavailable")
	}
	day := time.Date(2026, 3, 8, 0, 0, 0, 0, zone)
	tomorrow := day.AddDate(0, 0, 1)
	log := generateWorkLog([]task{{Title: "午夜计划", DueAt: tomorrow.UnixMilli()}}, day)
	if len(log.planned) != 1 {
		t.Fatal("DST day used a fixed 24-hour boundary")
	}
}
