// Package spike holds disposable fixtures only. It is not the product Todo service.
package spike

import (
	"strings"
	"time"

	"sidelet/internal/todo"
)

type Todo = todo.Todo
type Snapshot = todo.Snapshot
type Action = todo.Action

func New(now time.Time) Snapshot {
	titles := []string{"准备周会材料", "回复客户的产品反馈", "检查新版本构建", "整理本周设计记录", "确认接口联调时间", "阅读 20 分钟", "预约周末体检", "提交发布前检查清单"}
	due := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, now.Location())
	result := Snapshot{Todos: make([]Todo, len(titles)), Storage: "memory"}
	for i, title := range titles {
		description := "这是用于验证桌面交互的假数据。\n所有修改仅保留在本次运行中。"
		if i == 0 {
			description = "整理最新数据\n确认演示流程\n和团队对齐本周进度"
		}
		result.Todos[i] = Todo{ID: int64(i + 1), Title: title, Description: description, DisplayMode: "EDGE", StackID: 1}
		if i < 3 {
			result.Todos[i].DueAt = due.Add(time.Duration(i) * time.Hour).UnixMilli()
		}
	}
	return result
}
func Apply(s *Snapshot, action Action, now time.Time) {
	if action.Type == "reset" {
		*s = New(now)
		return
	}
	if action.Type == "select" {
		s.SelectedID = action.ID
		s.OverflowIDs = nil
		return
	}
	if action.Type == "undo" {
		if now.UnixMilli() <= s.UndoUntil {
			for i := range s.Todos {
				if s.Todos[i].ID == s.UndoID {
					s.Todos[i].Completed = false
					s.Todos[i].CompletedAt = 0
				}
			}
		}
		s.UndoID = 0
		s.UndoUntil = 0
		return
	}
	for i := range s.Todos {
		todo := &s.Todos[i]
		if todo.ID != action.ID {
			continue
		}
		switch action.Type {
		case "complete":
			if !todo.Completed {
				todo.Completed = true
				todo.CompletedAt = now.UnixMilli()
				s.UndoID = todo.ID
				s.UndoUntil = now.Add(5 * time.Second).UnixMilli()
			}
		case "snooze":
			until := now.Add(30 * time.Minute)
			if action.Duration == "1h" {
				until = now.Add(time.Hour)
			}
			if action.Duration == "tomorrow" {
				tomorrow := now.AddDate(0, 0, 1)
				until = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, now.Location())
			}
			todo.SnoozedUntil = until.UnixMilli()
		case "edit":
			if title := strings.TrimSpace(action.Title); title != "" {
				todo.Title = title
				todo.Description = action.Description
			}
		}
	}
}
