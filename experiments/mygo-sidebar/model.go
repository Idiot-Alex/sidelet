package main

import (
	"github.com/egoist/mygo"
	"sidelet/internal/settings"
	"sidelet/internal/todo"
	"time"
	"unicode/utf8"
)

const (
	stackWidth  = 312
	stackHeight = 166
	rowPitch    = 50
	cardWidth   = 320
	cardHeight  = 238
)

type task struct {
	DueAt          int64  `json:"dueAt,omitempty"`
	Remind         bool   `json:"remind,omitempty"`
	ReminderID     int64  `json:"reminderId,omitempty"`
	ReminderAt     int64  `json:"reminderAt,omitempty"`
	ReminderSentAt int64  `json:"reminderSentAt,omitempty"`
	Temporary      bool   `json:"temporary,omitempty"`
	CompletedAt    int64  `json:"completedAt,omitempty"`
	Title          string `json:"title"`
	Note           string `json:"note"`
	Priority       int    `json:"priority"`
	Done           bool   `json:"done"`
	Version        uint64 `json:"version"`
	Deleted        bool   `json:"deleted,omitempty"`
	SnoozedUntil   int64  `json:"snoozedUntil,omitempty"`
	Unpinned       bool   `json:"unpinned,omitempty"`
}

// Owns an optional independent experiment profile; never the formal store.
type model struct {
	profile                             *profile
	storageError                        string
	Preferences                         settings.Value `json:"preferences"`
	Quiet                               bool           `json:"quiet,omitempty"`
	ReminderGeneration                  int64          `json:"reminderGeneration,omitempty"`
	UndoID                              int            `json:"undoId,omitempty"`
	UndoUntil                           int64          `json:"undoUntil,omitempty"`
	Offset                              float64        `json:"offset"`
	ItemHeight                          int            `json:"itemHeight,omitempty"`
	clock                               func() time.Time
	EditError                           string `json:"editError"`
	draftVersion                        uint64
	Tasks                               []task `json:"tasks"`
	Order                               []int  `json:"order,omitempty"`
	Arranging                           bool   `json:"arranging,omitempty"`
	OverflowOpen                        bool   `json:"overflowOpen,omitempty"`
	Hover                               int    `json:"hover"`
	Opened                              int    `json:"opened"`
	Editing                             bool   `json:"editing"`
	Draft                               string `json:"draft"`
	DraftNote                           string `json:"draftNote"`
	UITheme                             string `json:"uiTheme,omitempty"`
	CardReadHeight                      int    `json:"cardReadHeight,omitempty"`
	Dragging                            bool   `json:"dragging"`
	Side                                string `json:"side"`
	X, Y                                int
	WorkX, WorkY, WorkWidth, WorkHeight int
	dragX, dragY, pointerX, pointerY    int
}

func newModel() *model {
	return &model{Preferences: settings.Defaults(), Tasks: []task{
		{Title: "整理今天的工作", Note: "这是独立实验中的合成任务。", Priority: 0, Version: 1},
		{Title: "确认设计稿", Note: "检查中文输入、卡片编辑和取消。", Priority: 2, Version: 1},
		{Title: "发布前检查", Note: "验证透明区域和窗口焦点。", Priority: 3, Version: 1},
	}, Hover: -1, Opened: -1, Side: "left", Offset: .35}
}

func (m *model) open(i int) {
	if m.eligible(i) && !m.Arranging && !m.Quiet {
		m.OverflowOpen = false
		m.Opened, m.Hover, m.Editing = i, i, false
	}
}

func (m *model) edit() {
	if m.Opened >= 0 {
		m.Editing, m.Draft = true, m.Tasks[m.Opened].Title
		m.DraftNote = m.Tasks[m.Opened].Note
		m.draftVersion, m.EditError = m.Tasks[m.Opened].Version, ""
	}
}

func (m *model) cancel() { m.Editing, m.Draft, m.DraftNote, m.EditError = false, "", "", "" }

func (m *model) save() bool {
	if m.Opened < 0 || !m.Editing {
		return false
	}
	title, err := validModelTitle(m, m.Draft)
	if err != nil {
		m.EditError = err.Error()
		return false
	}
	if m.Tasks[m.Opened].Version != m.draftVersion {
		m.EditError = "任务已更新，请取消后重新编辑。"
		return false
	}
	if m.UITheme != "" && utf8.RuneCountInString(m.DraftNote) > 16000 {
		m.EditError = "备注最多 16000 个字。"
		return false
	}
	if err := m.commit(func() error {
		m.Tasks[m.Opened].Title = title
		if m.UITheme != "" {
			m.Tasks[m.Opened].Note = m.DraftNote
		}
		m.Tasks[m.Opened].Version++
		m.cancel()
		return nil
	}); err != nil {
		m.EditError = err.Error()
		return false
	}
	return true
}

func (m *model) close() { m.Opened, m.Hover = -1, -1; m.OverflowOpen = false; m.cancel() }

func (m *model) complete() bool {
	if m.Opened < 0 {
		return false
	}
	if err := m.commit(func() error {
		m.recordCompletion(m.Opened)
		m.Tasks[m.Opened].Done = true
		m.Tasks[m.Opened].Version++
		m.close()
		return nil
	}); err != nil {
		m.EditError = err.Error()
		return false
	}
	return true
}

func (m *model) startDrag(x, y int) {
	if m.Quiet {
		return
	}
	m.Dragging = true
	m.dragX, m.dragY, m.pointerX, m.pointerY = m.X, m.Y, x, y
}

func (m *model) moveDrag(x, y int) {
	if !m.Dragging {
		return
	}
	m.X = m.dragX + x - m.pointerX
	m.Y = max(m.WorkY, min(m.WorkY+max(0, m.WorkHeight-m.stackHeight()), m.dragY+y-m.pointerY))
}

func (m *model) endDrag() bool {
	if !m.Dragging {
		return false
	}
	err := m.commit(func() error {
		m.Dragging = false
		if m.X+stackWidth/2 < m.WorkX+m.WorkWidth/2 {
			m.Side, m.X = "left", m.WorkX
		} else {
			m.Side, m.X = "right", m.WorkX+m.WorkWidth-stackWidth
		}
		if m.WorkHeight > 0 {
			center := m.stackHeight() / 2
			if m.UITheme != "" {
				center = 10 + int(m.sidebarLayout().Height)/2
				if m.desktopArrange() {
					center += 42
				}
			}
			m.Offset = float64(m.Y-m.WorkY+center) / float64(m.WorkHeight)
		}
		return nil
	})
	if err != nil {
		m.cancelDrag()
		m.EditError = err.Error()
		return false
	}
	return true
}

func (m *model) cancelDrag() {
	if m.Dragging {
		m.Dragging = false
		m.X, m.Y = m.dragX, m.dragY
	}
}

func (m *model) rowFor(index int) int {
	if m.UITheme != "" {
		direct, _ := m.sidebarItems()
		for row, i := range direct {
			if i == index {
				return row
			}
		}
		return len(direct)
	}
	row := 0
	for i := 0; i < index; i++ {
		if m.sidebarVisible(i) {
			row++
		}
	}
	return row
}

func (m *model) activeCount() int {
	n := 0
	for _, t := range m.Tasks {
		if !t.Done && !t.Deleted {
			n++
		}
	}
	return n
}

func (m *model) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}
func (m *model) recordCompletion(index int) {
	if m.UITheme != "" && !m.Tasks[index].Done {
		m.Tasks[index].CompletedAt = m.now().UnixMilli()
		m.UndoID = index + 1
		m.UndoUntil = m.now().Add(5 * time.Second).UnixMilli()
	}
}
func (m *model) cleanup() bool {
	changed := false
	for i, t := range m.Tasks {
		if !t.Deleted && todo.TemporaryExpired(t.Temporary, t.Done, t.CompletedAt, m.now()) {
			m.Tasks[i] = task{Deleted: true, Version: t.Version + 1}
			if m.UndoID == i+1 {
				m.UndoID = 0
				m.UndoUntil = 0
			}
			changed = true
		}
	}
	return changed
}
func (m *model) itemHeight() int {
	if m.UITheme != "" && m.ItemHeight > 0 {
		return m.ItemHeight
	}
	return 44
}
func (m *model) rowPitch() int { return m.itemHeight() + 6 }
func (m *model) totalCount() int {
	n := 0
	for _, t := range m.Tasks {
		if !t.Deleted {
			n++
		}
	}
	return n
}
func (m *model) sidebarVisible(i int) bool {
	if !m.eligible(i) {
		return false
	}
	if m.UITheme == "" {
		return true
	}
	direct, _ := m.sidebarItems()
	for _, index := range direct {
		if index == i {
			return true
		}
	}
	return false
}

// Offset is the stack's centre in the whole work area, as in stackLayout.
func (m *model) positionStack() {
	if m.WorkHeight <= 0 {
		return
	}
	height := m.stackHeight()
	if m.UITheme != "" {
		body := m.sidebarLayout().Height
		top := 10
		if m.desktopArrange() {
			top = 52
		}
		m.Y = max(m.WorkY, min(m.WorkY+max(0, m.WorkHeight-height), m.WorkY+int(float64(m.WorkHeight)*m.Offset)-top-int(body/2)))
		m.X = m.WorkX
		if m.Side == "right" {
			m.X = m.WorkX + m.WorkWidth - stackWidth
		}
		return
	}
	m.Y = max(m.WorkY+2, min(m.WorkY+m.WorkHeight-height-2, m.WorkY+int(float64(m.WorkHeight)*m.Offset)-height/2))
	m.X = m.WorkX
	if m.Side == "right" {
		m.X = m.WorkX + m.WorkWidth - stackWidth
	}
}
func (m *model) sidebarCount() int {
	n := 0
	for i := range m.Tasks {
		if m.eligible(i) {
			n++
		}
	}
	return n
}
func (m *model) undoVisible() bool {
	return !m.Quiet && m.UITheme != "" && m.UndoID > 0 && m.now().UnixMilli() < m.UndoUntil
}
func (m *model) undoRect() mygo.Rectangle {
	x := 8
	if m.Side == "right" {
		x = stackWidth - 144
	}
	y := 16 + m.sidebarCount()*m.rowPitch()
	if m.UITheme != "" {
		y = 20 + int(m.sidebarLayout().Height)
	}
	return mygo.Rectangle{X: x, Y: y, Width: 136, Height: 32}
}
func (m *model) stackHeight() int {
	if m.UITheme != "" {
		h := 20 + int(m.sidebarLayout().Height)
		if m.desktopArrange() {
			h += 132
		} else if m.undoVisible() {
			h += 46
		}
		if m.WorkHeight > 0 {
			h = min(h, m.WorkHeight)
		}
		return max(1, h)
	}
	h := 16 + m.sidebarCount()*m.rowPitch()
	if m.undoVisible() {
		h += 46
	}
	return h
}

func (m *model) sidebarCapacity() int {
	if m.WorkHeight > 0 {
		return min(8, max(1, (m.WorkHeight-16)/m.rowPitch()))
	}
	return 8
}
