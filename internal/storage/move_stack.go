package storage

import (
	"context"
	"math"
	"time"

	"sidelet/internal/todo"
)

// MoveStack changes only position. Compare the original layout inside the same
// transaction so a cancelled/stale preview cannot overwrite a newer layout.
func (s *Store) MoveStack(before, next todo.EdgeStack, now time.Time) (todo.Snapshot, error) {
	if before.ID <= 0 || next.ID != before.ID || next.DisplayID == "" || next.DisplayID != before.DisplayID || (next.Side != "left" && next.Side != "right") || math.IsNaN(next.Offset) || math.IsInf(next.Offset, 0) || next.Offset < 0 || next.Offset > 1 {
		return todo.Snapshot{}, validationError("桌面位置无效，请重新拖动。")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer tx.Rollback()
	state, err := load(tx)
	if err != nil {
		return todo.Snapshot{}, err
	}
	found := false
	for _, current := range state.Stacks {
		if current.ID == before.ID {
			found = current == before
		} else if current.DisplayID == next.DisplayID && current.Side == next.Side {
			return todo.Snapshot{}, validationError("这一侧已有另一组任务，请选择另一侧。")
		}
	}
	if !found {
		return todo.Snapshot{}, validationError("桌面位置已变化，请重新拖动。")
	}
	if next.Side != before.Side || next.Offset != before.Offset {
		// Density, task rows, ordering, and Undo are deliberately not written here.
		if _, err = tx.Exec(`UPDATE edge_stacks SET display_id=?,side=?,offset=?,updated_at=? WHERE id=?`, next.DisplayID, next.Side, next.Offset, now.UnixMilli(), before.ID); err != nil {
			return todo.Snapshot{}, err
		}
		state, err = load(tx)
		if err != nil {
			return todo.Snapshot{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return todo.Snapshot{}, err
	}
	return s.viewState(state, now), nil
}
