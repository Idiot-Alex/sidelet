package storage

import (
	"database/sql"
	"slices"

	"sidelet/internal/todo"
)

// Only the visible, unfinished members exchange existing sort slots. Hidden
// and completed members retain their exact slot and all task content is intact.
func reorder(tx *sql.Tx, action todo.Action, stamp int64) error {
	if action.StackID <= 0 || len(action.Order) == 0 {
		return validationError("请选择需要整理的桌面任务。")
	}
	rows, err := tx.Query(`SELECT i.todo_id,i.sort_order FROM edge_stack_items i
		JOIN todos t ON t.id=i.todo_id JOIN desktop_presentations p ON p.todo_id=t.id
		WHERE i.stack_id=? AND p.mode='EDGE' AND t.completed=0
		AND (t.snoozed_until IS NULL OR t.snoozed_until<=?) ORDER BY i.sort_order,i.todo_id`, action.StackID, stamp)
	if err != nil {
		return err
	}
	var current []int64
	var slots []int
	for rows.Next() {
		var id int64
		var slot int
		if err = rows.Scan(&id, &slot); err != nil {
			rows.Close()
			return err
		}
		current = append(current, id)
		slots = append(slots, slot)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !slices.Equal(current, action.PreviousOrder) || len(current) != len(action.Order) {
		return validationError("任务列表已变化，请按当前顺序重新拖动。")
	}
	remaining := make(map[int64]bool, len(current))
	for _, id := range current {
		remaining[id] = true
	}
	for _, id := range action.Order {
		if !remaining[id] {
			return validationError("排序包含重复任务或其他桌面的任务。")
		}
		delete(remaining, id)
	}
	if slices.Equal(current, action.Order) {
		return nil
	}
	for i := 1; i < len(slots); i++ {
		if slots[i] <= slots[i-1] {
			return validationError("桌面排序数据异常，未保存本次调整。")
		}
	}
	for i, id := range action.Order {
		if id == current[i] {
			continue
		}
		if _, err = tx.Exec(`UPDATE edge_stack_items SET sort_order=? WHERE stack_id=? AND todo_id=?`, slots[i], action.StackID, id); err != nil {
			return err
		}
	}
	return nil
}
