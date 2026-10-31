package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func navigationTester(t *testing.T) (*model, *nativeTasksView, *ui.Tester) {
	t.Helper()
	m := newModel()
	now := time.Date(2026, 10, 31, 10, 0, 0, 0, time.Local)
	m.UITheme = "mac"
	m.clock = func() time.Time { return now }
	m.Tasks = nil
	for i := range 48 {
		due := now.Add(time.Duration(i) * time.Minute)
		if i < 12 {
			due = now.AddDate(0, 0, -1).Add(time.Duration(i) * time.Minute)
		}
		m.Tasks = append(m.Tasks, task{Title: fmt.Sprintf("导航任务 %02d", i), Note: "用于长列表滚动与编辑的合成任务", DueAt: due.UnixMilli(), Version: 1, Done: i >= 24, CompletedAt: now.Add(-time.Duration(i) * time.Minute).UnixMilli()})
	}
	v := newNativeTasksView(testService(m))
	u := ui.NewTester(v.render, 860, 620)
	u.SetFocused(true)
	return m, v, u
}

func TestTimelineToolbarStaysVisibleAndEachFilterKeepsScroll(t *testing.T) {
	_, v, u := navigationTester(t)
	search := visualRect(t, u, "搜索任务")
	tab := visualRect(t, u, "待办 24")
	body := visualRect(t, u, "任务时间轴")
	u.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 650)
	pendingY := v.currentTaskScroll().Y
	if pendingY <= 0 {
		t.Fatal("task list did not scroll")
	}
	if visualRect(t, u, "搜索任务") != search || visualRect(t, u, "待办 24") != tab {
		t.Fatal("toolbar moved with list")
	}
	if err := u.Click("已完成 24"); err != nil {
		t.Fatal(err)
	}
	if v.currentTaskScroll().Y != 0 {
		t.Fatal("new category inherited previous offset")
	}
	u.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 300)
	completedY := v.currentTaskScroll().Y
	if completedY <= 0 {
		t.Fatal("completed list did not scroll")
	}
	if err := u.Click("全部 48"); err != nil {
		t.Fatal(err)
	}
	if v.currentTaskScroll().Y != 0 {
		t.Fatal("all category inherited another offset")
	}
	if err := u.Click("待办 24"); err != nil {
		t.Fatal(err)
	}
	if v.currentTaskScroll().Y != pendingY {
		t.Fatal("pending scroll not restored", v.currentTaskScroll().Y, pendingY)
	}
	if err := u.Click("已完成 24"); err != nil {
		t.Fatal(err)
	}
	if v.currentTaskScroll().Y != completedY {
		t.Fatal("completed scroll not restored")
	}
}

func TestTimelineCollapseSearchAndJumpPreserveEditor(t *testing.T) {
	_, v, u := navigationTester(t)
	click := func(label string) {
		t.Helper()
		if err := u.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	click("收起日期分组：已逾期")
	if u.HasText("导航任务 00") {
		t.Fatal("collapsed group still renders task rows")
	}
	click("收起日期分组：今天")
	if u.HasText("导航任务 12") {
		t.Fatal("today did not collapse")
	}
	click("回到今天")
	if !u.HasText("导航任务 12") {
		t.Fatal("jump did not expand today")
	}
	click("编辑：导航任务 12")
	u.Command("selectAll")
	u.Type("未保存的编辑草稿")
	click("收起日期分组：今天")
	if v.draft != "未保存的编辑草稿" || v.editID != 13 {
		t.Fatal("folding discarded editor draft")
	}
	click("搜索任务")
	u.Type("导航任务 00")
	if !u.HasText("导航任务 00") {
		t.Fatal("search failed to reveal a collapsed match")
	}
	click("清空搜索")
	if u.HasText("导航任务 00") {
		t.Fatal("clearing search lost remembered collapse state")
	}
	click("全部 48")
	if !u.HasText("导航任务 00") {
		t.Fatal("collapse state leaked across categories")
	}
}

func TestTodayJumpFindsOffscreenGroup(t *testing.T) {
	_, v, u := navigationTester(t)
	if err := u.Click("回到今天"); err != nil {
		t.Fatal(err)
	}
	body := visualRect(t, u, "任务时间轴")
	today := visualRect(t, u, "收起日期分组：今天")
	if v.currentTaskScroll().Y <= 0 || today.Y < body.Y-1 || today.Y+today.H > body.Y+body.H {
		t.Fatal("today remains outside viewport", today, body)
	}
	v.searchText = "不存在的任务"
	u.Frame()
	if v.hasTodayTasks() {
		t.Fatal("jump should not target a filtered-out group")
	}
}

func TestCollapsedDateIdentitySurvivesMidnight(t *testing.T) {
	m, _, u := navigationTester(t)
	if err := u.Click("已完成 24"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("收起日期分组：今天"); err != nil {
		t.Fatal(err)
	}
	now := m.now().AddDate(0, 0, 1)
	m.clock = func() time.Time { return now }
	u.Frame()
	if _, ok := u.Find("展开日期分组：昨天"); !ok {
		t.Fatal("collapsed date did not follow calendar rollover")
	}
	m.Tasks = append(m.Tasks, task{Title: "新一天完成的任务", Done: true, CompletedAt: now.UnixMilli(), Version: 1})
	u.Frame()
	if !u.HasText("新一天完成的任务") {
		t.Fatal("yesterday collapse leaked into a new today")
	}
}

func TestEditorScrollKeepsTaskToolbarAndSaveActionsVisible(t *testing.T) {
	_, v, u := navigationTester(t)
	if err := u.Click("编辑：导航任务 00"); err != nil {
		t.Fatal(err)
	}
	v.optionsOpen = true
	u.SetSize(820, 600)
	search := visualRect(t, u, "搜索任务")
	save := visualRect(t, u, "保存修改")
	cancel := visualRect(t, u, "取消编辑")
	fields := visualRect(t, u, "任务编辑内容")
	if save.Y+save.H > 600 || cancel.Y+cancel.H > 600 {
		t.Fatal("editor actions outside minimum window", save, cancel)
	}
	u.Scroll(fields.X+fields.W/2, fields.Y+fields.H/2, 0, 400)
	if visualRect(t, u, "保存修改") != save || visualRect(t, u, "取消编辑") != cancel || visualRect(t, u, "搜索任务") != search {
		t.Fatal("editor scroll moved toolbar or actions")
	}
	body := visualRect(t, u, "任务时间轴")
	u.Scroll(body.X+body.W/2, body.Y+body.H/2, 0, 500)
	if v.currentTaskScroll().Y <= 0 || visualRect(t, u, "保存修改") != save {
		t.Fatal("task scroll moved editor")
	}
	if err := u.Click("取消编辑"); err != nil {
		t.Fatal(err)
	}
	if v.editID != 0 {
		t.Fatal("pinned cancel action did not work")
	}
}
