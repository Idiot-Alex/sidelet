// Package storage persists user content independently of the native windows.
package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"sidelet/internal/todo"
)

const SchemaVersion = 2

//go:embed migrations/001_initial.sql
var initialSchema string

//go:embed migrations/002_reminders.sql
var reminderSchema string

type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	undoID    int64
	undoUntil int64
}

func DefaultDirectory() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Sidelet"), nil
}

func Open(directory string, now time.Time) (*Store, error) {
	return OpenWithDefaults(directory, now, "right", "normal")
}

func OpenWithDefaults(directory string, now time.Time, side, density string) (*Store, error) {
	if (side != "left" && side != "right") || (density != "compact" && density != "normal" && density != "relaxed") {
		return nil, errors.New("invalid Stack defaults")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(filepath.Join(directory, "sidelet.sqlite3"))
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	params := url.Values{}
	for _, pragma := range []string{"foreign_keys(1)", "busy_timeout(3000)", "journal_mode(WAL)", "synchronous(FULL)"} {
		params.Add("_pragma", pragma)
	}
	uri.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err := s.migrate(now, side, density); err != nil {
		db.Close()
		return nil, fmt.Errorf("open task database: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(now time.Time, side, density string) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	var version, count, userVersion int
	if err = tx.QueryRow(`SELECT COALESCE(MAX(version),0), COUNT(*) FROM schema_migrations`).Scan(&version, &count); err != nil {
		return err
	}
	if err = tx.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return err
	}
	if version > SchemaVersion || userVersion > SchemaVersion {
		return fmt.Errorf("database schema is newer than this application (migration=%d, user_version=%d)", version, userVersion)
	}
	if version != count || userVersion != version {
		return errors.New("inconsistent database migration history")
	}
	if version == 0 {
		if _, err = tx.Exec(initialSchema); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)`, now.UnixMilli()); err != nil {
			return err
		}
		if _, err = tx.Exec(`PRAGMA user_version = 1`); err != nil {
			return err
		}
	}
	if version < 2 {
		if _, err = tx.Exec(reminderSchema); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(2, ?)`, now.UnixMilli()); err != nil {
			return err
		}
		if _, err = tx.Exec(`PRAGMA user_version = 2`); err != nil {
			return err
		}
	}
	// An empty installation gets a layout, never sample user tasks.
	if _, err = tx.Exec(`INSERT INTO edge_stacks(display_id, side, offset, density, created_at, updated_at)
		SELECT '', ?, 0.35, ?, ?, ? WHERE NOT EXISTS(SELECT 1 FROM edge_stacks)`, side, density, now.UnixMilli(), now.UnixMilli()); err != nil {
		return err
	}
	// Revision 4 specifies cleanup on restart even inside the former Undo window.
	if _, err = tx.Exec(`DELETE FROM todos WHERE temporary=1 AND completed=1`); err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	invalid := rows.Next()
	rowsErr := rows.Err()
	rows.Close()
	if invalid {
		return errors.New("database contains broken task/layout references")
	}
	if rowsErr != nil {
		return rowsErr
	}
	if _, err = load(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func load(q queryer) (todo.Snapshot, error) {
	state := todo.Snapshot{Storage: "sqlite", Todos: []todo.Todo{}, Stacks: []todo.EdgeStack{}, OverflowIDs: []int64{}}
	rows, err := q.Query(`SELECT t.id,t.title,t.description,COALESCE(t.due_at,0),t.completed,
		COALESCE(t.completed_at,0),COALESCE(t.snoozed_until,0),t.priority,t.temporary,t.created_at,t.updated_at,
		p.mode,COALESCE(i.stack_id,0),COALESCE(i.sort_order,0),t.remind,
 COALESCE((SELECT remind_at FROM reminders WHERE todo_id=t.id ORDER BY id DESC LIMIT 1),0),
 COALESCE((SELECT delivered_at FROM reminders WHERE todo_id=t.id ORDER BY id DESC LIMIT 1),0)
		FROM todos t LEFT JOIN desktop_presentations p ON p.todo_id=t.id
		LEFT JOIN edge_stack_items i ON i.todo_id=t.id
		ORDER BY COALESCE(i.stack_id,0),COALESCE(i.sort_order,0),t.created_at,t.id`)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var item todo.Todo
		var mode sql.NullString
		err = rows.Scan(&item.ID, &item.Title, &item.Description, &item.DueAt, &item.Completed, &item.CompletedAt, &item.SnoozedUntil, &item.Priority, &item.Temporary, &item.CreatedAt, &item.UpdatedAt, &mode, &item.StackID, &item.SortOrder, &item.Remind, &item.RemindAt, &item.ReminderSentAt)
		if err != nil {
			rows.Close()
			return state, err
		}
		if !mode.Valid || (mode.String == "EDGE") != (item.StackID != 0) {
			rows.Close()
			return state, errors.New("inconsistent task desktop presentation")
		}
		item.DisplayMode = mode.String
		state.Todos = append(state.Todos, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	rows, err = q.Query(`SELECT id,display_id,side,offset,density,created_at,updated_at FROM edge_stacks ORDER BY id`)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var stack todo.EdgeStack
		if err = rows.Scan(&stack.ID, &stack.DisplayID, &stack.Side, &stack.Offset, &stack.Density, &stack.CreatedAt, &stack.UpdatedAt); err != nil {
			return state, err
		}
		state.Stacks = append(state.Stacks, stack)
	}
	return state, rows.Err()
}

func (s *Store) viewState(state todo.Snapshot, now time.Time) todo.Snapshot {
	if now.UnixMilli() <= s.undoUntil {
		state.UndoID = s.undoID
		state.UndoUntil = s.undoUntil
	}
	return state
}

func (s *Store) Snapshot(now time.Time) (todo.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Both lists belong to one database snapshot even with another reader/writer.
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer tx.Rollback()
	state, err := load(tx)
	if err != nil {
		return state, err
	}
	if err = tx.Commit(); err != nil {
		return state, err
	}
	return s.viewState(state, now), nil
}

func ItemHeight(density string) int {
	switch density {
	case "compact":
		return 40
	case "relaxed":
		return 56
	default:
		return 44
	}
}

func (s *Store) SaveStack(stack todo.EdgeStack, now time.Time) (todo.Snapshot, error) {
	if (stack.Side != "left" && stack.Side != "right") || math.IsNaN(stack.Offset) || math.IsInf(stack.Offset, 0) || stack.Offset < 0 || stack.Offset > 1 {
		return todo.Snapshot{}, errors.New("invalid desktop layout")
	}
	if stack.Density != "compact" && stack.Density != "normal" && stack.Density != "relaxed" {
		return todo.Snapshot{}, errors.New("invalid desktop density")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE edge_stacks SET display_id=?,side=?,offset=?,density=?,updated_at=? WHERE id=?`, stack.DisplayID, stack.Side, stack.Offset, stack.Density, now.UnixMilli(), stack.ID)
	if err != nil {
		return todo.Snapshot{}, err
	}
	if err = requireOne(result); err != nil {
		return todo.Snapshot{}, err
	}
	state, err := load(tx)
	if err != nil {
		return state, err
	}
	if err = tx.Commit(); err != nil {
		return state, err
	}
	return s.viewState(state, now), nil
}

func requireOne(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("task or desktop layout no longer exists")
	}
	return nil
}
