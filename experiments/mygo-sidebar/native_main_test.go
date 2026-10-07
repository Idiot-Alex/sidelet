package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func nativeTester(t *testing.T) (*model, *nativeTasksView, *ui.Tester) {
	t.Helper()
	m := newModel()
	v := newNativeTasksView(testService(m))
	u := ui.NewTester(v.renderLab, 760, 578)
	u.SetFocused(true)
	return m, v, u
}

func TestNativeMainAddEditCancelAndPriority(t *testing.T) {
	m, v, u := nativeTester(t)
	u.Type("原生新增任务")
	if err := u.Click("新任务重要程度"); err != nil {
		t.Fatal(err)
	}
	u.Key(0, ui.KeyEnd)
	u.Key(0, ui.KeyEnter)
	if err := u.Click("添加"); err != nil {
		t.Fatal(err)
	}
	if len(m.Tasks) != 4 || m.Tasks[3].Priority != 3 || m.Tasks[3].Title != "原生新增任务" || !u.Focused("新任务标题") {
		t.Fatalf("add did not preserve focus or priority: %+v", m.Tasks)
	}
	if err := u.Click("编辑：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if !u.Focused("编辑任务标题") {
		t.Fatal("edit title not focused")
	}
	u.Command("selectAll")
	u.Compose("中文", 2)
	u.Type("中文原生编辑")
	v.draftPriority = "重要"
	if err := u.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Title != "中文原生编辑" || m.Tasks[0].Priority != 2 || v.editID != 0 {
		t.Fatal("save did not reach shared model")
	}
	if err := u.Click("编辑：中文原生编辑"); err != nil {
		t.Fatal(err)
	}
	u.Command("selectAll")
	u.Type("不应该保存")
	u.Key(0, ui.KeyEscape)
	if m.Tasks[0].Title != "中文原生编辑" || v.editID != 0 {
		t.Fatal("cancel changed committed task")
	}
}

func TestNativeMainIMEKeepsDraftAcrossOtherTaskMutationAndConflict(t *testing.T) {
	m, v, u := nativeTester(t)
	if err := u.Click("编辑：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	u.Command("selectAll")
	u.Compose("正在输入", 4)
	_, _ = v.service.Update(2, "另一任务更新", 3, 1)
	u.SetSize(700, 550)
	if _, active := u.TextCaret(); !u.Focused("编辑任务标题") || !active {
		t.Fatal("unrelated mutation or resize reset composition")
	}
	u.Type("我的草稿")
	m.open(0)
	m.edit()
	m.Draft = "侧边卡片新内容"
	if !m.save() {
		t.Fatal("native card save failed")
	}
	if err := u.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if !v.failed || v.editID != 1 || v.draft != "我的草稿" || m.Tasks[0].Title != "侧边卡片新内容" {
		t.Fatal("stale main draft lost or overwrote card save")
	}
	if err := u.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("编辑：侧边卡片新内容"); err != nil {
		t.Fatal(err)
	}
	if v.draft != m.Tasks[0].Title || !u.Focused("编辑任务标题") {
		t.Fatal("re-edit did not reload and focus latest title")
	}
}

func TestNativeMainCompletionRestoreAndCard(t *testing.T) {
	m, v, u := nativeTester(t)
	opened := -1
	v.service.open = func(i int) { opened = i }
	if err := u.Click("打开卡片：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if opened != 0 || m.Opened != 0 {
		t.Fatal("card did not share task")
	}
	if err := u.Click("完成：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if !m.Tasks[0].Done || m.Opened != -1 || m.stackHeight() != 116 {
		t.Fatal("complete did not compact sidebar and close card")
	}
	if err := u.Click("已完成 1"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("恢复：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Done || m.stackHeight() != 166 || u.HasText("整理今天的工作") {
		t.Fatal("restore did not update sidebar/filter")
	}
	if err := u.Click("待办 3"); err != nil {
		t.Fatal(err)
	}
	if !u.HasText("整理今天的工作") {
		t.Fatal("restored task missing")
	}
}

func TestNativeMainScrollAndCompletionWhileEditing(t *testing.T) {
	m, v, u := nativeTester(t)
	for m.activeCount() < 8 {
		_, _ = v.service.Add("待办列表中的任务", 0)
	}
	u.SetSize(560, 438)
	u.Scroll(300, 310, 0, 300)
	if v.scroll.Y <= 0 || v.scroll.MaxY <= 0 {
		t.Fatal("bounded list did not scroll at minimum window size")
	}
	v.setFilter(false)
	v.beginEdit(v.service.snapshot().Tasks[0])
	u.SetSize(560, 438)
	_, _ = v.service.SetDone(1, true, 1)
	u.SetSize(560, 438)
	if v.editID != 0 || v.draft != "" || m.Tasks[0].Done != true {
		t.Fatal("completion retained an editor for a hidden task")
	}
}
