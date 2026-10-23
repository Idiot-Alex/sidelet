package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// NativeProfile stores committed MyGo state separately from the legacy task
// schema. Task IDs are positional and tombstones are retained to prevent reuse.
// Metadata and task rows commit together; JSON encodes evolving domain fields,
// while SQLite owns durability, locking and atomic transactions.
type NativeProfile struct{ db *sql.DB }

const NativeProfileFilename = "sidelet-native.sqlite3"
const nativeApplicationID = 0x534c4e50

func OpenNativeProfile(dir string) (*NativeProfile, error) {
	path := filepath.Join(dir, NativeProfileFilename)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("native database must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := sqliteFileURI(abs)
	q := url.Values{}
	for _, p := range []string{"busy_timeout(3000)", "synchronous(FULL)"} {
		q.Add("_pragma", p)
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ok := false
	defer func() {
		if !ok {
			db.Close()
		}
	}()
	var id, version, tables int
	if err = db.QueryRow("PRAGMA application_id").Scan(&id); err != nil {
		return nil, err
	}
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return nil, err
	}
	if id != nativeApplicationID && (id != 0 || version != 0 || tables != 0) {
		return nil, errors.New("not a Sidelet native database")
	}
	if version > 2 || (id == nativeApplicationID && version != 1 && version != 2) {
		return nil, errors.New("unsupported native database version")
	}
	if id == 0 {
		tx, e := db.Begin()
		if e != nil {
			return nil, e
		}
		defer tx.Rollback()
		if _, e = tx.Exec(fmt.Sprintf(`CREATE TABLE native_profile (id INTEGER PRIMARY KEY CHECK(id=1), metadata BLOB NOT NULL);
CREATE TABLE native_tasks (id INTEGER PRIMARY KEY CHECK(id>0), payload BLOB NOT NULL);
PRAGMA application_id=%d; PRAGMA user_version=1;`, nativeApplicationID)); e != nil {
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
	}
	if version < 2 {
		tx, e := db.Begin()
		if e != nil {
			return nil, e
		}
		defer tx.Rollback()
		if _, e = tx.Exec(`CREATE TABLE native_new_task_draft (id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL);
PRAGMA user_version=2;`); e != nil {
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
	}
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	ok = true
	return &NativeProfile{db}, nil
}
func (p *NativeProfile) Close() error { return p.db.Close() }
func (p *NativeProfile) Read() ([]byte, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var meta []byte
	err = tx.QueryRow("SELECT metadata FROM native_profile WHERE id=1").Scan(&meta)
	if errors.Is(err, sql.ErrNoRows) {
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM native_tasks").Scan(&n); e != nil {
			return nil, e
		}
		if n != 0 {
			return nil, errors.New("orphan native tasks")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(meta, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("invalid native metadata")
	}
	var draft []byte
	if err = tx.QueryRow("SELECT payload FROM native_new_task_draft WHERE id=1").Scan(&draft); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	delete(fields, "newTaskDraft")
	if draft != nil {
		if err = validateNativeDraft(draft); err != nil {
			return nil, err
		}
		fields["newTaskDraft"] = json.RawMessage(draft)
	}
	rows, err := tx.Query("SELECT id,payload FROM native_tasks ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []json.RawMessage{}
	size := len(meta)
	for rows.Next() {
		var id int
		var data []byte
		if err = rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		size += len(data)
		if id != len(tasks)+1 || size > 64<<20 || !json.Valid(data) {
			return nil, errors.New("invalid native task rows")
		}
		tasks = append(tasks, json.RawMessage(data))
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	fields["tasks"], err = json.Marshal(tasks)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}
func (p *NativeProfile) Write(_ string, data []byte) error {
	if len(data) > 64<<20 {
		return errors.New("native profile too large")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(fields["tasks"], &tasks); err != nil {
		return err
	}
	delete(fields, "tasks")
	draft := fields["newTaskDraft"]
	delete(fields, "newTaskDraft")
	if err := validateNativeDraft(draft); err != nil {
		return err
	}
	meta, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO native_profile(id,metadata) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET metadata=excluded.metadata", meta); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO native_tasks(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload WHERE payload != excluded.payload")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, t := range tasks {
		if _, err = stmt.Exec(i+1, []byte(t)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("DELETE FROM native_tasks WHERE id>?", len(tasks)); err != nil {
		return err
	}
	if err = writeNativeDraft(tx, draft); err != nil {
		return err
	}
	return tx.Commit()
}

func validateNativeDraft(data []byte) error {
	if len(data) > 1<<20 || len(data) > 0 && !json.Valid(data) {
		return errors.New("invalid native draft")
	}
	return nil
}

func writeNativeDraft(tx *sql.Tx, data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		_, err := tx.Exec("DELETE FROM native_new_task_draft WHERE id=1")
		return err
	}
	_, err := tx.Exec("INSERT INTO native_new_task_draft(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload WHERE payload != excluded.payload", data)
	return err
}

// WriteDraft touches only the singleton draft row, never the task list.
// Write includes that row in the task transaction to consume a draft atomically.
func (p *NativeProfile) WriteDraft(data []byte) error {
	if err := validateNativeDraft(data); err != nil {
		return err
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = writeNativeDraft(tx, data); err != nil {
		return err
	}
	return tx.Commit()
}
