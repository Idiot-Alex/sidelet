package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"reflect"
	"slices"
	"testing"
	"time"
)

func orderFixture(t *testing.T, count int) (*model, *TasksService) {
	t.Helper()
	m := newModel()
	m.UITheme = "mac"
	m.WorkWidth, m.WorkHeight = 1728, 1000
	s := testService(m)
	for len(m.Tasks) < count {
		if _, err := s.AddWithPin(fmt.Sprintf("额外任务 %d", len(m.Tasks)+1), "备注", 0, true); err != nil {
			t.Fatal(err)
		}
	}
	return m, s
}
func TestNativeOverflowAllowsAddPinUndoReopenAndUnsnooze(t *testing.T) {
	m, s := orderFixture(t, 12)
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	_, _ = s.SetDone(1, true, m.Tasks[0].Version)
	_, _ = s.AddWithPin("完成后填满", "", 0, true)
	if _, err := s.Undo(); err != nil || m.Tasks[0].Done {
		t.Fatal("full sidebar rejected undo", err)
	}
	_, _ = s.SetDone(2, true, m.Tasks[1].Version)
	if _, err := s.SetDone(2, false, m.Tasks[1].Version); err != nil {
		t.Fatal("full sidebar rejected reopen", err)
	}
	_, _ = s.Snooze(3, "1h", m.Tasks[2].Version)
	if _, err := s.Unsnooze(3, m.Tasks[2].Version); err != nil {
		t.Fatal("full sidebar rejected unsnooze", err)
	}
	_, _ = s.SetPinned(4, false, m.Tasks[3].Version)
	if _, err := s.SetPinned(4, true, m.Tasks[3].Version); err != nil {
		t.Fatal("full sidebar rejected pin", err)
	}
	direct, tail := m.sidebarItems()
	if len(direct) != 8 || len(tail) != 5 || m.stackHeight() != 464 || len(stackInputRegions(m)) != 9 {
		t.Fatal("overflow geometry lost tasks")
	}
	all := append(slices.Clone(direct), tail...)
	slices.Sort(all)
	for i, id := range all {
		if id != i {
			t.Fatal("duplicate or missing task", all)
		}
	}
}
func TestNativeReorderPreservesHiddenSlotsContentVersionsUndoAndDraft(t *testing.T) {
	m, s := orderFixture(t, 10)
	now := time.Unix(1000, 0)
	m.clock = func() time.Time { return now }
	_, _ = s.SetDone(4, true, m.Tasks[3].Version)
	_, _ = s.Snooze(5, "1h", m.Tasks[4].Version)
	_, _ = s.SetPinned(6, false, m.Tasks[5].Version)
	_, _ = s.Delete(7, m.Tasks[6].Version)
	before := slices.Clone(m.Tasks)
	undoID, undoUntil := m.UndoID, m.UndoUntil
	v := newNativeTasksView(s)
	v.beginEdit(s.List().Tasks[0])
	v.draft = "未提交的草稿"
	_, _ = s.Arrange(true)
	previous := m.eligibleIDs()
	if _, err := s.Reorder(previous, moveOrder(previous, 10, 1, false)); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Order, []int{10, 1, 2, 4, 5, 6, 7, 3, 8, 9}) {
		t.Fatal("hidden slot moved", m.Order)
	}
	if !reflect.DeepEqual(before, m.Tasks) || m.UndoID != undoID || m.UndoUntil != undoUntil || v.draft != "未提交的草稿" || v.editVersion != before[0].Version {
		t.Fatal("sort changed task payload or references")
	}
	_, _ = s.Arrange(false)
	if _, err := s.Undo(); err != nil || !slices.Equal(m.eligibleIDs(), []int{10, 1, 2, 4, 3, 8, 9}) {
		t.Fatal("undo did not restore original hidden slot", err, m.eligibleIDs())
	}
	if _, err := s.UpdateDetails(1, v.draft, "原始草稿", 0, v.editVersion); err != nil {
		t.Fatal("sort invalidated existing draft", err)
	}
}
func TestNativeReorderRejectsStaleDuplicateAndInvalidOrdersAtomically(t *testing.T) {
	m, s := orderFixture(t, 10)
	if _, err := s.Reorder(m.eligibleIDs(), m.eligibleIDs()); err == nil {
		t.Fatal("sort outside arrange accepted")
	}
	_, _ = s.Arrange(true)
	previous := m.eligibleIDs()
	for _, invalid := range [][]int{{1}, {1, 1, 3, 4, 5, 6, 7, 8, 9, 10}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 999}} {
		if _, err := s.Reorder(previous, invalid); err == nil || m.Order != nil {
			t.Fatal("invalid order changed state")
		}
	}
	_, _ = s.Reorder(previous, moveOrder(previous, 10, 1, false))
	committed := slices.Clone(m.Order)
	if _, err := s.Reorder(previous, previous); err == nil || !slices.Equal(m.Order, committed) {
		t.Fatal("stale order overwrote committed sort")
	}
	previous = m.eligibleIDs()
	_, _ = s.Snooze(1, "1h", m.Tasks[0].Version)
	if _, err := s.Reorder(previous, previous); err == nil {
		t.Fatal("hidden member allowed stale drop")
	}
}
func TestNativeMainOrderKeyboardDragEscapeAndProtectedForm(t *testing.T) {
	for _, theme := range []string{"mac", "paper", "graphite"} {
		t.Run(theme, func(t *testing.T) {
			m, v, u := webUITester(t)
			m.UITheme = theme
			v.visual = webTheme(theme)
			_ = u.Click("编辑：整理今天的工作")
			v.draft = "保留编辑草稿"
			if err := u.Click("整理桌面"); err != nil {
				t.Fatal(err)
			}
			if !m.Arranging || !u.HasText("桌面任务顺序") || v.editID != 1 {
				t.Fatal("arrange lost editor")
			}
			_ = u.Click("编辑任务标题")
			u.Type("不能输入")
			_ = u.Click("保存修改")
			if v.draft != "保留编辑草稿" || m.Tasks[0].Title != "整理今天的工作" {
				t.Fatal("arrange editor not disabled")
			}
			_ = u.Click("拖动排序整理今天的工作")
			u.Key(ui.Alt, ui.KeyDown)
			if !slices.Equal(m.eligibleIDs(), []int{2, 1, 3}) || !u.Focused("拖动排序整理今天的工作") {
				t.Fatal("keyboard sort lost source focus", m.eligibleIDs())
			}
			h, _ := u.Find("拖动排序发布前检查")
			row, _ := u.Find("拖动排序确认设计稿")
			u.Press(h.X+10, h.Y+h.H/2)
			u.Move(row.X+80, row.Y+2)
			u.Release(row.X+80, row.Y+2)
			if !slices.Equal(m.eligibleIDs(), []int{3, 2, 1}) {
				t.Fatal("pointer drop did not reorder", m.eligibleIDs())
			}
			h, _ = u.Find("拖动排序发布前检查")
			row, _ = u.Find("拖动排序整理今天的工作")
			u.Press(h.X+10, h.Y+h.H/2)
			u.Move(row.X+80, row.Y+row.H-2)
			u.Key(0, ui.KeyEscape)
			u.Release(row.X+80, row.Y+row.H-2)
			if !m.Arranging || !slices.Equal(m.eligibleIDs(), []int{3, 2, 1}) {
				t.Fatal("Escape drag cancelled order or exited arrange")
			}
			u.Key(0, ui.KeyEscape)
			if m.Arranging || v.draft != "保留编辑草稿" || v.editID != 1 {
				t.Fatalf("arrange Escape lost draft: arranging=%v draft=%q edit=%d text=%v", m.Arranging, v.draft, v.editID, u.HasText("保留编辑草稿"))
			}
		})
	}
}
func TestNativeOrderDropCancelledByBlurResizeAndMembershipChange(t *testing.T) {
	m, v, u := webUITester(t)
	_ = u.Click("整理桌面")
	drag := func(change func()) {
		h, _ := u.Find("拖动排序整理今天的工作")
		r, _ := u.Find("拖动排序发布前检查")
		u.Press(h.X+10, h.Y+10)
		u.Move(r.X+80, r.Y+r.H-2)
		change()
		u.Frame()
		u.Release(r.X+80, r.Y+r.H-2)
	}
	drag(func() { v.order.Epoch++; u.SetFocused(false); u.SetFocused(true) })
	if m.Order != nil {
		t.Fatal("blur allowed stale drop")
	}
	drag(func() { v.order.Epoch++; u.SetSize(1000, 780) })
	if m.Order != nil {
		t.Fatal("resize allowed stale drop")
	}
	drag(func() { _, _ = v.service.SetPinned(2, false, m.Tasks[1].Version) })
	if m.Order != nil || !m.Tasks[1].Unpinned {
		t.Fatal("membership change allowed stale drop")
	}
}
func TestNativeOverflowCardChooseRevealsWithoutChangingStoredOrder(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		t.Run(side, func(t *testing.T) {
			m, s := orderFixture(t, 12)
			m.Side = side
			m.positionStack()
			v := &views{m: m, service: s}
			u := ui.NewTester(v.stack, 312, m.stackHeight())
			if err := u.Click("查看其余4项任务"); err != nil {
				t.Fatal(err)
			}
			if !m.OverflowOpen || !v.session.Open {
				t.Fatal("overflow did not open card session")
			}
			card := ui.NewTester(v.card, 320, 390)
			if !card.HasText("更多任务") || !card.HasText("额外任务 12") {
				t.Fatal("overflow list missing")
			}
			if err := card.Click("查看任务：额外任务 12"); err != nil {
				t.Fatal(err)
			}
			card.Frame()
			u.Frame()
			direct, tail := m.sidebarItems()
			if m.OverflowOpen || m.Opened != 11 || !slices.Equal(direct, []int{0, 1, 2, 3, 4, 5, 6, 11}) || !slices.Equal(tail, []int{7, 8, 9, 10}) || m.Order != nil {
				t.Fatal("choose changed stored order", direct, tail)
			}
			if !card.HasText("额外任务 12") || !card.HasText("编辑") || len(stackInputRegions(m)) != 10 {
				t.Fatal("chosen card or expanded input missing")
			}
			_ = card.Click("编辑")
			m.Draft = "来自溢出的编辑"
			_ = card.Click("保存")
			if m.Tasks[11].Title != "来自溢出的编辑" {
				t.Fatal("overflow card edit failed")
			}
			_ = card.Click("完成")
			if !m.Tasks[11].Done || m.cardOpen() {
				t.Fatal("overflow task completion failed")
			}
			if _, err := s.Undo(); err != nil {
				t.Fatal("overflow task undo rejected", err)
			}
		})
	}
}
func TestNativeOverflowShortScreenRegionsAndTransparency(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		for _, area := range []int{8, 40, 100, 180, 1000} {
			for _, density := range []int{40, 44, 56} {
				m, _ := orderFixture(t, 30)
				m.Side, m.WorkHeight, m.ItemHeight = side, area, density
				m.positionStack()
				direct, tail := m.sidebarItems()
				if len(direct)+len(tail) != 30 || len(direct) > 8 || m.stackHeight() > area {
					t.Fatal("layout exceeds work area")
				}
				for _, r := range stackInputRegions(m) {
					if r.rect.Y < 0 || r.rect.Y+r.rect.Height > m.stackHeight() {
						t.Fatal("input outside viewport", r)
					}
				}
				v := &views{m: m}
				u := ui.NewTester(v.stack, 312, m.stackHeight())
				img := u.Image()
				if area > 40 && img.RGBAAt(150, min(15, m.stackHeight()-1)).A != 0 {
					t.Fatal("blank backdrop painted")
				}
			}
		}
	}
}
func TestNativeDesktopOrderAndOverflowTailStayAccessible(t *testing.T) {
	m, s := orderFixture(t, 12)
	_, _ = s.Arrange(true)
	v := &views{m: m, service: s}
	u := ui.NewTester(v.stack, 312, m.stackHeight())
	u.SetFocused(true)
	if !u.HasText("全部任务") || !u.HasText("+4") || len(stackInputRegions(m)) != 11 {
		t.Fatal("desktop arrange controls missing")
	}
	if r := m.arrangeToolbarRect(); r.Y-(m.overflowRect().Y+m.overflowRect().Height) != 20 {
		t.Fatal("toolbar touches overflow row", r)
	}
	_ = u.Click("拖动排序整理今天的工作")
	u.Key(ui.Alt, ui.KeyDown)
	if !slices.Equal(m.eligibleIDs()[:3], []int{2, 1, 3}) {
		t.Fatal("desktop keyboard sort failed")
	}
	var opened bool
	v.showAll = func() { opened = true }
	_ = u.Click("查看其余4项任务")
	if !opened || m.OverflowOpen {
		t.Fatal("arranging +N did not open full sort view")
	}
	_ = u.Click("完成整理")
	if m.Arranging {
		t.Fatal("desktop finish did not exit")
	}
}

func TestNativeArrangeGroupDragPreservesBodyCenterAndLayoutGuard(t *testing.T) {
	m, s := orderFixture(t, 12)
	m.WorkX, m.WorkY = -100, 24
	m.Offset = .4
	_, _ = s.Arrange(true)
	m.positionStack()
	m.startDrag(m.X+20, m.Y+20)
	m.moveDrag(m.X+20, m.Y+100)
	bodyCenter := m.Y - m.WorkY + 52 + int(m.sidebarLayout().Height)/2
	m.endDrag()
	beforeY := m.Y
	if m.Offset != float64(bodyCenter)/float64(m.WorkHeight) {
		t.Fatal("group drag used toolbar center", m.Offset)
	}
	m.positionStack()
	if m.Y != beforeY {
		t.Fatal("release moved ordered rows", beforeY, m.Y)
	}
	side := m.Side
	if _, err := s.Layout("left", .5, 56); err == nil || m.Side != side || m.itemHeight() != 44 {
		t.Fatal("layout changed during arrange")
	}
	_, _ = s.Arrange(false)
	if _, err := s.Layout("left", .5, 56); err != nil {
		t.Fatal(err)
	}
}
