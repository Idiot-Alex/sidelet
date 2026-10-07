package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func webUITester(t *testing.T) (*model, *nativeTasksView, *ui.Tester) {
	t.Helper()
	m := newModel()
	m.UITheme = "mac"
	v := newNativeTasksView(testService(m))
	u := ui.NewTester(v.render, 1120, 778)
	u.SetFocused(true)
	return m, v, u
}

func TestFormalUIEditorNotesIMEAndConflict(t *testing.T) {
	m, v, u := webUITester(t)
	if err := u.Click("编辑：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if !u.Focused("编辑任务标题") || v.formNote != m.Tasks[0].Note {
		t.Fatal("editor did not load note and focus")
	}
	u.Command("selectAll")
	u.Compose("中文", 2)
	_, _ = v.service.UpdateDetails(2, "独立更新", "其他备注", 2, 1)
	u.SetSize(850, 600)
	if _, active := u.TextCaret(); !active || !u.Focused("编辑任务标题") {
		t.Fatal("resize or mutation interrupted composition")
	}
	u.Type("中文正式布局")
	v.formNote = "多行备注\n第二行"
	if err := u.Click("保存修改"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Title != "中文正式布局" || m.Tasks[0].Note != "多行备注\n第二行" || v.editID != 0 {
		t.Fatalf("title and note not saved together: task=%+v edit=%d draft=%q note=%q status=%q", m.Tasks[0], v.editID, v.draft, v.formNote, v.status)
	}
	if err := u.Click("编辑：中文正式布局"); err != nil {
		t.Fatal(err)
	}
	u.Command("selectAll")
	u.Type("不能覆盖的新草稿")
	v.formNote = "不能覆盖的备注"
	_, _ = v.service.UpdateDetails(1, "卡片已更新", "卡片的备注", 3, m.Tasks[0].Version)
	u.SetSize(1120, 778)
	if err := u.Click("保存修改"); err != nil {
		t.Fatal(err)
	}
	if !v.failed || v.draft != "不能覆盖的新草稿" || v.formNote != "不能覆盖的备注" || m.Tasks[0].Title != "卡片已更新" {
		t.Fatal("stale submission lost draft or overwrote latest task")
	}
	if err := u.Click("取消编辑"); err != nil {
		t.Fatal(err)
	}
	if v.editID != 0 || v.formNote != "" {
		t.Fatal("cancel leaked old note into new task")
	}
}

func TestFormalUIAllFilterCanEditCompletedTask(t *testing.T) {
	m, v, u := webUITester(t)
	_, _ = v.service.SetDone(1, true, 1)
	u.SetSize(1120, 778)
	if err := u.Click("全部 3"); err != nil {
		t.Fatal(err)
	}
	if err := u.Click("编辑：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	u.SetSize(850, 600)
	if v.editID != 1 {
		t.Fatal("all filter cancelled completed editor")
	}
	u.Command("selectAll")
	u.Type("完成后仍可编辑")
	if err := u.Click("保存修改"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Title != "完成后仍可编辑" || !m.Tasks[0].Done {
		t.Fatalf("completed task update failed: task=%+v edit=%d draft=%q status=%q", m.Tasks[0], v.editID, v.draft, v.status)
	}
}

func TestFormalUIPinDoesNotOpenCardAndUnpinnedDoesNotConsumeCapacity(t *testing.T) {
	m, v, u := webUITester(t)
	if err := u.Click("取消桌面固定：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if !m.Tasks[0].Unpinned || m.Opened != -1 || m.stackHeight() != 114 || len(stackInputRegions(m)) != 2 {
		t.Fatal("pin action opened a card or left its input region")
	}
	if err := u.Click("固定到桌面：整理今天的工作"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Unpinned || m.stackHeight() != 164 {
		t.Fatal("re-pin did not restore sidebar")
	}
	for m.sidebarCount() < 8 {
		if _, err := v.service.Add("桌面任务", 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := v.service.AddWithPin("未固定任务", "", 0, false); err != nil {
		t.Fatal(err)
	}
	if m.activeCount() != 9 || m.sidebarCount() != 8 {
		t.Fatal("unpinned task consumed sidebar capacity")
	}
	if _, err := v.service.SetPinned(9, true, 1); err != nil {
		t.Fatal("overflow pin rejected", err)
	}
	direct, tail := m.sidebarItems()
	if len(direct) != 8 || len(tail) != 1 || m.sidebarCount() != 9 {
		t.Fatal("pin did not enter overflow")
	}
}

func TestFormalUIThemesAndSettings(t *testing.T) {
	m, v, u := webUITester(t)
	if err := u.Click("设置"); err != nil {
		t.Fatal(err)
	}
	if r, ok := u.Find("外观设置"); !ok || r.W != 850 {
		t.Fatalf("appearance panel must share full settings width: %+v", r)
	}
	for _, theme := range []struct{ label, id string }{{"温暖纸色", "paper"}, {"深色石墨", "graphite"}, {"精致 Mac", "mac"}} {
		if err := u.Click(theme.label); err != nil {
			t.Fatal(err)
		}
		if m.UITheme != theme.id || v.visual.ID != theme.id {
			t.Fatal("theme not shared with overlay model")
		}
	}
	u.Scroll(600, 600, 0, 1800)
	if !u.HasText("导出任务") || !u.HasText("快速添加") {
		t.Fatal("settings sections missing")
	}
	if err := u.Click("返回我的任务"); err != nil {
		t.Fatal(err)
	}
	if v.settingsOpen || !u.HasText("我的任务") {
		t.Fatal("cannot return to task page")
	}
}

func TestFormalUITransparentRegionsAndExpandedGripBothSides(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		for _, theme := range []string{"mac", "paper", "graphite"} {
			m := newModel()
			m.UITheme, m.Side = theme, side
			v := &views{m: m}
			u := ui.NewTester(v.stack, stackWidth, stackHeight)
			img := u.Image()
			for _, p := range [][2]int{{140, 30}, {140, 80}, {7, 55}, {310, 165}} {
				if img.RGBAAt(p[0], p[1]).A != 0 {
					t.Fatalf("%s/%s painted blank %v", side, theme, p)
				}
			}
			x := float32(7)
			if side == "right" {
				x = 305
			}
			u.Move(x, 30)
			v.hoverDeadline = time.Unix(1, 0)
			u.Frame()
			u.Move(140, 30)
			if m.Hover != 0 || !u.HasText(m.Tasks[0].Title) {
				t.Fatalf("%s/%s lost preview on entry", side, theme)
			}
			regions := stackInputRegions(m)
			if len(regions) != 4 || regions[0].rect.Width != 20 || regions[1].rect.Width != 276 {
				t.Fatal("expanded hit region differs from painted grip")
			}
			if u.Image().RGBAAt(140, 80).A != 0 {
				t.Fatal("non-hovered row gained background")
			}
		}
	}
}

func TestFormalUIHoverDelayAndLeaveBridge(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	v := &views{m: m}
	now := time.Unix(1000, 0)
	if next, delay := v.webHover(0, now); next != -1 || delay != 150*time.Millisecond {
		t.Fatal("entry not delayed by 150ms")
	}
	if next, _ := v.webHover(0, now.Add(149*time.Millisecond)); next != -1 {
		t.Fatal("revealed before entry delay")
	}
	m.Hover, _ = v.webHover(0, now.Add(150*time.Millisecond))
	if m.Hover != 0 {
		t.Fatal("entry did not reveal")
	}
	if next, delay := v.webHover(-1, now.Add(time.Second)); next != 0 || delay != 500*time.Millisecond {
		t.Fatal("leave did not preserve 500ms bridge")
	}
	if next, _ := v.webHover(0, now.Add(1200*time.Millisecond)); next != 0 {
		t.Fatal("re-entry collapsed preview")
	}
	if next, delay := v.webHover(-1, now.Add(2*time.Second)); next != 0 || delay != 500*time.Millisecond {
		t.Fatal("re-entry did not reset leave delay")
	}
	if next, _ := v.webHover(-1, now.Add(2500*time.Millisecond)); next != -1 {
		t.Fatal("leave did not collapse after delay")
	}
}

func TestFormalUICardNotesCancelAndValidation(t *testing.T) {
	m := newModel()
	m.UITheme = "paper"
	m.open(0)
	v := &views{m: m}
	u := ui.NewTester(v.card, 320, 390)
	u.SetFocused(true)
	if err := u.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	u.Command("selectAll")
	u.Type("卡片标题")
	m.DraftNote = "卡片备注\n第二行"
	if err := u.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Title != "卡片标题" || m.Tasks[0].Note != "卡片备注\n第二行" {
		t.Fatal("card note not saved")
	}
	if err := u.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	m.Draft = "过期"
	m.DraftNote = "过期备注"
	m.Tasks[0].Version++
	if err := u.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if !m.Editing || m.EditError == "" || m.Tasks[0].Title != "卡片标题" {
		t.Fatal("stale card update accepted")
	}
	if err := u.Click("取消"); err != nil {
		t.Fatal(err)
	}
	if m.Editing || m.Opened != 0 || m.Tasks[0].Note != "卡片备注\n第二行" {
		t.Fatal("cancel did not preserve read card")
	}
	m.edit()
	m.DraftNote = strings.Repeat("字", 16001)
	if m.save() || !m.Editing || m.Tasks[0].Title != "卡片标题" {
		t.Fatal("oversize note partially committed")
	}
}

func TestFormalUIQuickAddAndTitleLimit(t *testing.T) {
	m := newModel()
	m.UITheme = "mac"
	q := &nativeQuickAdd{service: testService(m), text: "旧草稿", error: "旧错误", pin: true}
	q.reset()
	if q.text != "" || q.error != "" || q.pin || !q.focus {
		t.Fatal("reopening quick add retained previous draft/options")
	}
	u := ui.NewTester(q.render, 540, 260)
	u.SetFocused(true)
	u.Compose("中文", 2)
	u.Type("快速添加验收")
	u.Key(0, ui.KeyEnter)
	if len(m.Tasks) != 4 || m.Tasks[3].Title != "快速添加验收" || !m.Tasks[3].Unpinned || m.sidebarCount() != 3 {
		t.Fatal("quick add did not save as an unpinned task")
	}
	if _, err := q.service.AddWithPin(strings.Repeat("字", 500), "", 0, false); err != nil {
		t.Fatal(err)
	}
	before := len(m.Tasks)
	if _, err := q.service.AddWithPin(strings.Repeat("字", 501), "", 0, false); err == nil || len(m.Tasks) != before {
		t.Fatal("production title limit was not enforced atomically")
	}
}
