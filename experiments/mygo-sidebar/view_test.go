package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestHoverCrossesIntoPreviewAndLeaves(t *testing.T) {
	m := newModel()
	v := &views{m: m}
	u := ui.NewTester(v.stack, stackWidth, stackHeight)
	if m.Hover != -1 || u.HasText(m.Tasks[0].Title) {
		t.Fatal("idle reveals a task")
	}
	u.Move(7, 30)
	if m.Hover != 0 || !u.HasText(m.Tasks[0].Title) {
		t.Fatal("handle did not reveal task")
	}
	u.Move(140, 30)
	if m.Hover != 0 || !u.HasText(m.Tasks[0].Title) {
		t.Fatal("preview vanished while entering it")
	}
	u.Move(140, 160)
	if m.Hover != -1 || u.HasText(m.Tasks[0].Title) {
		t.Fatal("preview did not collapse")
	}
	u.Move(7, 80)
	if m.Hover != 1 || !u.HasText(m.Tasks[1].Title) || u.HasText(m.Tasks[0].Title) {
		t.Fatal("second marker reveals wrong task")
	}
}

func TestBlankAndRoundedAreasStayTransparent(t *testing.T) {
	m := newModel()
	v := &views{m: m}
	u := ui.NewTester(v.stack, stackWidth, stackHeight)
	img := u.Image()
	for _, p := range [][2]int{{140, 30}, {7, 55}, {310, 165}, {0, 0}} {
		if img.RGBAAt(p[0], p[1]).A != 0 {
			t.Fatalf("idle background at %v is painted", p)
		}
	}
	if img.RGBAAt(7, 30).A == 0 {
		t.Fatal("marker is invisible")
	}
	u.Move(7, 30)
	img = u.Image()
	if img.RGBAAt(140, 30).A == 0 || img.RGBAAt(140, 80).A != 0 {
		t.Fatal("only hovered row must be painted")
	}
	if img.RGBAAt(14, 8).A != 0 {
		t.Fatal("rounded corner is opaque")
	}
}

func TestCardChineseSaveCancelAndCompletion(t *testing.T) {
	m := newModel()
	m.open(0)
	v := &views{m: m}
	u := ui.NewTester(v.card, cardWidth, cardHeight)
	u.SetFocused(true)
	if err := u.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	if !m.Editing {
		t.Fatal("edit did not start")
	}
	u.Command("selectAll")
	u.Compose("中文输入", 4)
	u.Type("中文输入保存")
	if err := u.Click("保存"); err != nil {
		t.Fatal(err)
	}
	if m.Tasks[0].Title != "中文输入保存" || m.Editing {
		t.Fatalf("save failed: %+v", m)
	}
	if err := u.Click("编辑"); err != nil {
		t.Fatal(err)
	}
	u.Command("selectAll")
	u.Type("取消的草稿")
	u.Key(0, ui.KeyEscape)
	if m.Tasks[0].Title != "中文输入保存" || m.Editing || m.Opened != 0 {
		t.Fatal("Escape did not cancel to the reading card")
	}
	if err := u.Click("完成"); err != nil {
		t.Fatal(err)
	}
	if !m.Tasks[0].Done || m.Opened != -1 {
		t.Fatal("completion failed")
	}
}

func TestRepeatedEditorFocusAndSave(t *testing.T) {
	m := newModel()
	m.open(0)
	v := &views{m: m}
	u := ui.NewTester(v.card, cardWidth, cardHeight)
	u.SetFocused(true)
	for _, title := range []string{"第一轮", "第二轮", "第三轮"} {
		if err := u.Click("编辑"); err != nil {
			t.Fatal(err)
		}
		if !u.Focused("任务标题") {
			t.Fatal("reopened editor did not focus title")
		}
		u.Command("selectAll")
		u.Type(title)
		if err := u.Click("保存"); err != nil {
			t.Fatal(err)
		}
		if m.Tasks[0].Title != title {
			t.Fatalf("save got %q, want %q", m.Tasks[0].Title, title)
		}
	}
}

func TestDragClampsSnapsAndCancelsWithoutJumping(t *testing.T) {
	m := newModel()
	m.WorkWidth, m.WorkHeight = 1600, 1000
	m.Y = 350
	m.startDrag(7, 380)
	m.moveDrag(307, 430)
	if m.X != 300 || m.Y != 400 {
		t.Fatalf("delta wrong: %d,%d", m.X, m.Y)
	}
	// The second sample is measured against the original point, not the moved window.
	m.moveDrag(1207, 900)
	m.endDrag()
	if m.Side != "right" || m.X != 1288 || m.Y != 834 {
		t.Fatalf("snap/clamp wrong: %+v", m)
	}
	m.startDrag(1590, 850)
	m.moveDrag(10, -300)
	m.cancelDrag()
	if m.X != 1288 || m.Y != 834 || m.Dragging {
		t.Fatal("cancellation changed original position")
	}
}

func TestBlankTitleAndInvalidTaskAreIgnored(t *testing.T) {
	m := newModel()
	m.open(-1)
	m.open(9)
	if m.Opened != -1 {
		t.Fatal("invalid task opened")
	}
	m.open(1)
	m.edit()
	m.Draft = "  \n  "
	if m.save() || !m.Editing || m.Tasks[1].Title != "确认设计稿" {
		t.Fatal("blank title saved")
	}
}

func TestRightMarkerKeepsPreviewOpen(t *testing.T) {
	m := newModel()
	m.Side = "right"
	v := &views{m: m}
	u := ui.NewTester(v.stack, stackWidth, stackHeight)
	u.Move(305, 30)
	u.Move(140, 30)
	if m.Hover != 0 || !u.HasText(m.Tasks[0].Title) {
		t.Fatal("right-side transition failed")
	}
}
