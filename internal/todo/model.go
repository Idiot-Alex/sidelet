// Package todo defines the task and desktop state shared by storage and UI.
package todo

type Todo struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	DueAt          int64  `json:"dueAt"`
	Remind         bool   `json:"remind"`
	RemindAt       int64  `json:"remindAt"`
	ReminderSentAt int64  `json:"reminderSentAt"`
	Completed      bool   `json:"completed"`
	CompletedAt    int64  `json:"completedAt"`
	SnoozedUntil   int64  `json:"snoozedUntil"`
	Priority       int    `json:"priority"`
	Temporary      bool   `json:"temporary"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	// These fields are a projection of separate presentation/layout tables.
	DisplayMode string `json:"displayMode"`
	StackID     int64  `json:"stackId"`
	SortOrder   int    `json:"sortOrder"`
}

type EdgeStack struct {
	ID        int64   `json:"id"`
	DisplayID string  `json:"displayId"`
	Side      string  `json:"side"`
	Offset    float64 `json:"offset"`
	Density   string  `json:"density"`
	CreatedAt int64   `json:"createdAt"`
	UpdatedAt int64   `json:"updatedAt"`
}

type Snapshot struct {
	Todos       []Todo      `json:"todos"`
	Stacks      []EdgeStack `json:"stacks"`
	Storage     string      `json:"storage"`
	UndoID      int64       `json:"undoId"`
	UndoUntil   int64       `json:"undoUntil"`
	SelectedID  int64       `json:"selectedId"`
	OverflowIDs []int64     `json:"overflowIds"`
}

func (s Snapshot) Copy() Snapshot {
	s.Todos = append([]Todo{}, s.Todos...)
	s.Stacks = append([]EdgeStack{}, s.Stacks...)
	s.OverflowIDs = append([]int64{}, s.OverflowIDs...)
	return s
}

type Action struct {
	Type          string  `json:"type"`
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Duration      string  `json:"duration"`
	DueAt         *int64  `json:"dueAt,omitempty"`
	Priority      *int    `json:"priority,omitempty"`
	Temporary     *bool   `json:"temporary,omitempty"`
	Pin           bool    `json:"pin"`
	StackID       int64   `json:"stackId"`
	Remind        *bool   `json:"remind,omitempty"`
	RequestID     string  `json:"requestId,omitempty"`
	Order         []int64 `json:"order,omitempty"`
	PreviousOrder []int64 `json:"previousOrder,omitempty"`
}
