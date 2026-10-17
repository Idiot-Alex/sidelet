package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"sidelet/internal/todo"
)

// BackupLegacy reads the old database without running migrations or restart
// cleanup. VACUUM INTO includes committed WAL content in a standalone backup.
// The caller supplies a new path in a private backup directory.
func BackupLegacy(directory, backup string) (todo.Snapshot, error) {
	path, err := filepath.Abs(filepath.Join(directory, "sidelet.sqlite3"))
	if err != nil {
		return todo.Snapshot{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return todo.Snapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return todo.Snapshot{}, errors.New("旧数据库必须是普通文件。")
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	uri.RawQuery = "mode=ro&_pragma=busy_timeout(3000)"
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Never overwrite an earlier backup or accept the source as the output.
	if _, err = os.Lstat(backup); !os.IsNotExist(err) {
		return todo.Snapshot{}, errors.New("备份路径已存在或无法访问。")
	}
	if _, err = db.Exec("VACUUM main INTO ?", backup); err != nil {
		return todo.Snapshot{}, err
	}
	if err = os.Chmod(backup, 0600); err != nil {
		return todo.Snapshot{}, err
	}
	return ReadLegacyBackup(backup)
}

// ReadLegacyBackup validates and reads one consistent standalone snapshot.
func ReadLegacyBackup(path string) (todo.Snapshot, error) {
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	uri.RawQuery = "mode=ro&immutable=1"
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return todo.Snapshot{}, err
	}
	defer tx.Rollback()
	var version, maxVersion, count int
	if err = tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return todo.Snapshot{}, err
	}
	if err = tx.QueryRow("SELECT COALESCE(MAX(version),0),COUNT(*) FROM schema_migrations").Scan(&maxVersion, &count); err != nil {
		return todo.Snapshot{}, err
	}
	if version != SchemaVersion || maxVersion != version || count != version {
		return todo.Snapshot{}, errors.New("旧数据库版本不受支持或迁移记录不一致。")
	}
	var check string
	if err = tx.QueryRow("PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return todo.Snapshot{}, fmt.Errorf("旧数据库完整性检查失败：%s (%v)", check, err)
	}
	rows, err := tx.Query("PRAGMA foreign_key_check")
	if err != nil {
		return todo.Snapshot{}, err
	}
	broken := rows.Next()
	err = rows.Err()
	rows.Close()
	if broken || err != nil {
		return todo.Snapshot{}, errors.New("旧数据库的任务与布局引用无效。")
	}
	state, err := load(tx)
	if err != nil {
		return state, err
	}
	return state, tx.Commit()
}
