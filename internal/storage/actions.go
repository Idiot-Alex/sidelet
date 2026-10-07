package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"sidelet/internal/todo"
)

type validationError string

func (e validationError) Error() string { return string(e) }

func UserMessage(err error) string {
	var validation validationError
	if errors.As(err, &validation) {
		return validation.Error()
	}
	return "保存失败，请稍后重试。修改尚未写入。"
}

func validate(action todo.Action) error {
	if action.Type == "create" || action.Type == "edit" {
		title := strings.TrimSpace(action.Title)
		if title == "" {
			return validationError("任务标题不能为空。")
		}
		if utf8.RuneCountInString(title) > 500 || strings.ContainsAny(title, "\x00\r\n") {
			return validationError("标题最多 500 字，且不能包含换行。")
		}
		if len(action.Description) > 48000 {
			return validationError("备注过长，请缩短后重试。")
		}
	}
	if action.DueAt != nil {
		if err := todo.ValidateDeadline(*action.DueAt, action.Remind != nil && *action.Remind); err != nil {
			return validationError(err.Error())
		}
	}
	if action.Priority != nil && (*action.Priority < 0 || *action.Priority > 3) {
		return validationError("优先级无效。")
	}
	if action.Remind != nil && *action.Remind && action.DueAt != nil && *action.DueAt == 0 {
		return validationError("请先设置截止时间，再开启提醒。")
	}
	if action.Type == "snooze" && action.Duration != "30m" && action.Duration != "1h" && action.Duration != "tomorrow" {
		return validationError("稍后提醒时间无效。")
	}
	return nil
}

func nullable(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
func optionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
func optionalBool(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

// Apply commits content and presentation changes atomically, then returns the
// resulting state. UI/session fields are never written into SQLite.
func (s *Store) Apply(action todo.Action, now time.Time) (todo.Snapshot, error) {
	if err := validate(action); err != nil {
		return todo.Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer tx.Rollback()
	undoID, undoUntil := s.undoID, s.undoUntil
	if now.UnixMilli() > undoUntil {
		undoID = 0
		undoUntil = 0
	}
	stamp := now.UnixMilli()
	var result sql.Result
	switch action.Type {
	case "create":
		priority := 0
		temporary := false
		remind := action.Remind != nil && *action.Remind
		due := int64(0)
		if action.Priority != nil {
			priority = *action.Priority
		}
		if action.Temporary != nil {
			temporary = *action.Temporary
		}
		if action.DueAt != nil {
			due = *action.DueAt
		}
		if remind && due == 0 {
			return todo.Snapshot{}, validationError("请先设置截止时间，再开启提醒。")
		}
		result, err = tx.Exec(`INSERT INTO todos(title,description,due_at,priority,temporary,created_at,updated_at,remind) VALUES(?,?,?,?,?,?,?,?)`, strings.TrimSpace(action.Title), action.Description, nullable(due), priority, temporary, stamp, stamp, remind)
		if err != nil {
			return todo.Snapshot{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return todo.Snapshot{}, err
		}
		if id > 9007199254740991 {
			return todo.Snapshot{}, errors.New("task ID exceeds the UI integer range")
		}
		if _, err = tx.Exec(`INSERT INTO desktop_presentations(todo_id,mode,created_at,updated_at) VALUES(?,'NONE',?,?)`, id, stamp, stamp); err != nil {
			return todo.Snapshot{}, err
		}
		if action.Pin {
			if err = pin(tx, id, action.StackID, stamp); err != nil {
				return todo.Snapshot{}, err
			}
		}
	case "edit":
		var due any
		if action.DueAt != nil {
			due = nullable(*action.DueAt)
		}
		result, err = tx.Exec(`UPDATE todos SET title=?,description=?,due_at=CASE WHEN ? THEN ? ELSE due_at END,
			priority=COALESCE(?,priority),temporary=COALESCE(?,temporary),remind=CASE WHEN ? THEN 0 ELSE COALESCE(?,remind) END,updated_at=? WHERE id=?`,
			strings.TrimSpace(action.Title), action.Description, action.DueAt != nil, due, optionalInt(action.Priority), optionalBool(action.Temporary), action.DueAt != nil && *action.DueAt == 0, optionalBool(action.Remind), stamp, action.ID)
		if err == nil {
			err = requireOne(result)
		}
	case "complete":
		var completed bool
		if err = tx.QueryRow(`SELECT completed FROM todos WHERE id=?`, action.ID).Scan(&completed); err != nil {
			return todo.Snapshot{}, err
		}
		if !completed {
			_, err = tx.Exec(`UPDATE todos SET completed=1,completed_at=?,updated_at=? WHERE id=?`, stamp, stamp, action.ID)
			undoID = action.ID
			undoUntil = now.Add(5 * time.Second).UnixMilli()
		}
	case "undo":
		if undoID == 0 || stamp > undoUntil {
			return todo.Snapshot{}, validationError("撤销时间已结束。")
		}
		result, err = tx.Exec(`UPDATE todos SET completed=0,completed_at=NULL,updated_at=? WHERE id=? AND completed=1`, stamp, undoID)
		if err == nil {
			err = requireOne(result)
		}
		undoID = 0
		undoUntil = 0
	case "reopen":
		result, err = tx.Exec(`UPDATE todos SET completed=0,completed_at=NULL,updated_at=? WHERE id=?`, stamp, action.ID)
		if err == nil {
			err = requireOne(result)
		}
		if undoID == action.ID {
			undoID = 0
			undoUntil = 0
		}
	case "snooze":
		until, _ := todo.SnoozeUntil(now, action.Duration)
		result, err = tx.Exec(`UPDATE todos SET snoozed_until=?,updated_at=? WHERE id=?`, until.UnixMilli(), stamp, action.ID)
		if err == nil {
			err = requireOne(result)
		}
	case "unsnooze":
		result, err = tx.Exec(`UPDATE todos SET snoozed_until=NULL,updated_at=? WHERE id=?`, stamp, action.ID)
		if err == nil {
			err = requireOne(result)
		}
	case "pin":
		err = pin(tx, action.ID, action.StackID, stamp)
	case "unpin":
		result, err = tx.Exec(`UPDATE desktop_presentations SET mode='NONE',updated_at=? WHERE todo_id=?`, stamp, action.ID)
		if err == nil {
			err = requireOne(result)
		}
		if err == nil {
			_, err = tx.Exec(`DELETE FROM edge_stack_items WHERE todo_id=?`, action.ID)
		}
	case "reorder":
		err = reorder(tx, action, stamp)
	case "delete":
		result, err = tx.Exec(`DELETE FROM todos WHERE id=?`, action.ID)
		if err == nil {
			err = requireOne(result)
		}
		if undoID == action.ID {
			undoID = 0
			undoUntil = 0
		}
	case "cleanup":
		_, err = tx.Exec(`DELETE FROM todos WHERE temporary=1 AND completed=1 AND completed_at<?`, stamp-5000)
	default:
		return todo.Snapshot{}, validationError("不支持这项任务操作。")
	}
	if err != nil {
		return todo.Snapshot{}, err
	}
	if action.Remind != nil && *action.Remind && action.Type == "edit" {
		var due int64
		if err = tx.QueryRow(`SELECT COALESCE(due_at,0) FROM todos WHERE id=?`, action.ID).Scan(&due); err != nil {
			return todo.Snapshot{}, err
		}
		if due == 0 {
			return todo.Snapshot{}, validationError("请先设置截止时间，再开启提醒。")
		}
	}
	if err = syncReminders(tx, stamp); err != nil {
		return todo.Snapshot{}, err
	}
	state, err := load(tx)
	if err != nil {
		return state, err
	}
	if err = tx.Commit(); err != nil {
		return state, err
	}
	s.undoID = undoID
	s.undoUntil = undoUntil
	return s.viewState(state, now), nil
}

func pin(tx *sql.Tx, id, stackID, stamp int64) error {
	if stackID == 0 {
		if err := tx.QueryRow(`SELECT id FROM edge_stacks ORDER BY id LIMIT 1`).Scan(&stackID); err != nil {
			return err
		}
	}
	var target int64
	if err := tx.QueryRow(`SELECT id FROM edge_stacks WHERE id=?`, stackID).Scan(&target); err != nil {
		return validationError("目标桌面分组不存在。")
	}
	result, err := tx.Exec(`UPDATE desktop_presentations SET mode='EDGE',updated_at=? WHERE todo_id=?`, stamp, id)
	if err != nil {
		return err
	}
	if err = requireOne(result); err != nil {
		return err
	}
	var currentStack int64
	err = tx.QueryRow(`SELECT stack_id FROM edge_stack_items WHERE todo_id=?`, id).Scan(&currentStack)
	if err == nil && currentStack == stackID {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(`INSERT INTO edge_stack_items(stack_id,todo_id,sort_order)
		VALUES(?,?,(SELECT COALESCE(MAX(sort_order),0)+10 FROM edge_stack_items WHERE stack_id=?))
		ON CONFLICT(todo_id) DO UPDATE SET stack_id=excluded.stack_id,sort_order=excluded.sort_order`, stackID, id, stackID)
	return err
}
