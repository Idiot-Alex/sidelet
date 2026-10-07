package storage

import (
	"database/sql"
	"sidelet/internal/todo"
	"time"
)

// Alias preserves the storage API while allowing schedulers to stay independent of SQLite.
type Reminder = todo.Reminder

// Keep a stable request ID across title edits, completion/undo, and restarts.
// Changing the effective deadline or explicitly disabling/enabling creates a
// new generation; old OS notifications can then be removed by their old ID.
func syncReminders(tx *sql.Tx, stamp int64) error {
	_, err := tx.Exec(`DELETE FROM reminders WHERE NOT EXISTS (
 SELECT 1 FROM todos t WHERE t.id=reminders.todo_id AND t.remind=1 AND t.due_at>0
 AND reminders.remind_at=MAX(t.due_at,COALESCE(t.snoozed_until,0)))`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO reminders(todo_id,remind_at,created_at)
 SELECT t.id,MAX(t.due_at,COALESCE(t.snoozed_until,0)),? FROM todos t
 WHERE t.remind=1 AND t.due_at>0 AND NOT EXISTS(SELECT 1 FROM reminders r WHERE r.todo_id=t.id)`, stamp)
	return err
}

func (s *Store) Reminders() ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT r.id,r.todo_id,t.title,r.remind_at,COALESCE(r.delivered_at,0),t.completed
 FROM reminders r JOIN todos t ON t.id=r.todo_id WHERE t.remind=1 AND t.due_at>0 ORDER BY r.remind_at,r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reminders := []Reminder{}
	for rows.Next() {
		var r Reminder
		if err = rows.Scan(&r.ID, &r.TodoID, &r.Title, &r.At, &r.SentAt, &r.Completed); err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}
	return reminders, rows.Err()
}

func (s *Store) MarkReminderSent(id int64, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE reminders SET delivered_at=? WHERE id=? AND delivered_at IS NULL`, now.UnixMilli(), id)
	return err
}
