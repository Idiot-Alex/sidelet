package main

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTaskSearchMatchesTitleNoteAndCurrentCategory(t *testing.T) {
	v := newNativeTasksView(testService(newModel()))
	item := taskEntry{Title: "整理 Sidelet", Note: "明天评审", Done: true}
	for _, query := range []string{"sideLET", "  整理  ", "评审", ""} {
		v.searchQuery = query
		if v.taskVisible(item) {
			t.Fatal("completed task escaped pending category")
		}
		v.filterAll = true
		if !v.taskVisible(item) {
			t.Fatalf("missing match for %q", query)
		}
		v.filterAll = false
	}
	v.filterAll, v.searchQuery = true, "不存在"
	if v.taskVisible(item) {
		t.Fatal("unmatched task visible")
	}
	item.Deleted, v.searchQuery = true, ""
	if v.taskVisible(item) {
		t.Fatal("deleted task visible")
	}
}

func TestTaskSearchKeyboardIMEAndDraft(t *testing.T) {
	_, v, u := webUITester(t)
	v.beginEdit(v.service.snapshot().Tasks[0])
	v.draft = "尚未保存的修改"
	u.Frame()
	mods := ui.Ctrl
	if runtime.GOOS == "darwin" {
		mods = ui.Super
	}
	u.Key(mods, ui.KeyF)
	u.Frame()
	if !u.Focused("搜索任务") {
		t.Fatal("search shortcut did not focus field")
	}
	u.Compose("没有匹配", 4)
	if v.searchQuery != "" {
		t.Fatal("IME preedit filtered tasks")
	}
	u.Type("没有匹配")
	u.Frame()
	if !u.HasText("没有找到匹配的任务") || !u.HasText("0 项") {
		t.Fatal("missing empty search feedback")
	}
	if v.editID != 1 || v.draft != "尚未保存的修改" {
		t.Fatal("search lost editor draft")
	}
	u.Key(0, ui.KeyEscape)
	u.Frame()
	if v.searchText != "" || v.searchQuery != "" || v.editID == 0 {
		t.Fatal("Escape did not clear only search")
	}
	u.Type("没有匹配")
	if err := u.Click("清空搜索"); err != nil {
		t.Fatal(err)
	}
	u.Frame()
	if v.searchQuery != "" || !u.Focused("搜索任务") {
		t.Fatal("clear did not restore search focus")
	}
}

func TestTaskSearchLayoutStaysStableWhileTyping(t *testing.T) {
	_, v, u := webUITester(t)
	for _, width := range []int{1120, 820, 540} {
		u.SetSize(width, 778)
		v.searchText = ""
		u.Frame()
		before := visualRect(t, u, "搜索任务")
		rowBefore := visualRect(t, u, "编辑：整理今天的工作")
		if before.W < 200 {
			t.Fatalf("search input too narrow at %d: %v", width, before)
		}
		if err := u.Click("搜索任务"); err != nil {
			t.Fatal(err)
		}
		u.Type("整理")
		u.Frame()
		if after := visualRect(t, u, "搜索任务"); after != before {
			t.Fatalf("search field moved while typing: %v -> %v", before, after)
		}
		if after := visualRect(t, u, "编辑：整理今天的工作"); after.Y != rowBefore.Y {
			t.Fatal("result count moved task list", rowBefore, after)
		}
	}
}
