package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"sidelet/internal/settings"
	"sidelet/internal/storage"
	"sidelet/internal/todo"
)

type legacyImport struct {
	Source      string           `json:"source"`
	Backup      string           `json:"backup"`
	SHA256      string           `json:"sha256"`
	ImportedAt  int64            `json:"importedAt"`
	OriginalIDs []int64          `json:"originalIds"`
	Stacks      []todo.EdgeStack `json:"stacks"`
}

type legacyImportResult struct {
	Tasks           int    `json:"tasks"`
	Backup          string `json:"backup"`
	AlreadyImported bool   `json:"alreadyImported"`
}

func convertLegacy(snapshot todo.Snapshot, preferences settings.Value) (profileState, error) {
	if len(snapshot.Stacks) != 1 {
		return profileState{}, errors.New("目前仅支持迁移一个侧栏任务组；已保留旧库，请先整理任务组。")
	}
	stack := snapshot.Stacks[0]
	if stack.Density != "compact" && stack.Density != "normal" && stack.Density != "relaxed" {
		return profileState{}, errors.New("旧版标签密度无效。")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return profileState{}, err
	}
	v := profileState{Schema: 2, ID: hex.EncodeToString(id), Preferences: preferences, Theme: preferences.Appearance.Theme,
		Side: stack.Side, Offset: stack.Offset, ItemHeight: storage.ItemHeight(stack.Density), Tasks: []task{}}
	seen := map[int64]bool{}
	for _, old := range snapshot.Todos {
		if old.ID <= 0 || seen[old.ID] {
			return v, errors.New("旧版任务编号重复或无效。")
		}
		seen[old.ID] = true
		if old.DisplayMode != "NONE" && old.DisplayMode != "EDGE" || old.DisplayMode == "EDGE" && old.StackID != stack.ID {
			return v, errors.New("旧版任务布局无效。")
		}
		t := task{Title: old.Title, Note: old.Description, Version: 1, DueAt: old.DueAt, Remind: old.Remind,
			ReminderSentAt: old.ReminderSentAt, Temporary: old.Temporary, Done: old.Completed, CompletedAt: old.CompletedAt,
			SnoozedUntil: old.SnoozedUntil, Priority: old.Priority, Unpinned: old.DisplayMode != "EDGE", CreatedAt: old.CreatedAt, UpdatedAt: old.UpdatedAt}
		if old.Priority == 1 {
			t.Priority = 0
		} // Legacy normal priority; UI has three supported levels.
		if t.Remind && t.DueAt > 0 {
			v.ReminderGeneration++
			t.ReminderID, t.ReminderAt = v.ReminderGeneration, todo.EffectiveReminderAt(t.DueAt, t.SnoozedUntil)
			if old.RemindAt != t.ReminderAt {
				t.ReminderSentAt = 0
			}
		} else {
			t.ReminderSentAt = 0
		}
		v.Tasks = append(v.Tasks, t)
		v.Order = append(v.Order, len(v.Tasks))
	}
	return v, validateProfile(v)
}

// Explicit offline migration. A profile lock and one SQLite transaction keep
// source identity, tasks and reminder receipts committed together. Existing
// native tasks are never replaced or implicitly merged; an empty JSON profile
// is backed up and its ID retained.
func importLegacyProfile(source, destination string, now time.Time) (legacyImportResult, error) {
	var result legacyImportResult
	source, err := filepath.Abs(source)
	if err != nil {
		return result, err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return result, err
	}
	destination, err = profileDirectory(destination, "")
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(destination, 0700); err != nil {
		return result, err
	}
	f, err := os.OpenFile(filepath.Join(destination, "profile.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return result, err
	}
	defer f.Close()
	if err = lockProfile(f); err != nil {
		return result, errors.New("请先退出使用目标资料的 Sidelet。")
	}
	defer unlockProfile(f)
	db, err := storage.OpenNativeProfile(destination)
	if err != nil {
		return result, err
	}
	defer db.Close()
	data, err := db.Read()
	if err != nil {
		return result, err
	}
	var existing profileState
	if data != nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&existing); err != nil {
			return result, err
		}
		if err = validateProfile(existing); err != nil {
			return result, err
		}
		if existing.LegacyImport != nil {
			if existing.LegacyImport.Source != source {
				return result, errors.New("此资料已导入其他旧库，不能重复导入。")
			}
			return legacyImportResult{len(existing.LegacyImport.OriginalIDs), existing.LegacyImport.Backup, true}, nil
		}
		if len(existing.Tasks) != 0 {
			return result, errors.New("目标资料已有 MyGo 任务，已停止导入以避免覆盖。")
		}
	}
	var nativeJSON []byte
	jsonPath := filepath.Join(destination, "profile.json")
	if info, e := os.Lstat(jsonPath); e == nil {
		if !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return result, errors.New("目标 JSON 资料类型或大小无效。")
		}
		nativeJSON, err = os.ReadFile(jsonPath)
		if err != nil {
			return result, err
		}
		if data == nil {
			decoder := json.NewDecoder(bytes.NewReader(nativeJSON))
			decoder.DisallowUnknownFields()
			if err = decoder.Decode(&existing); err != nil {
				return result, err
			}
			if err = validateProfile(existing); err != nil {
				return result, err
			}
			if len(existing.Tasks) != 0 || existing.LegacyImport != nil {
				return result, errors.New("目标 JSON 已有任务或迁移记录，不能覆盖。")
			}
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	preferences := settings.Open(source)
	if preferences.LoadError != nil {
		return result, fmt.Errorf("旧版设置无法读取：%w", preferences.LoadError)
	}
	backupRoot := filepath.Join(destination, "legacy-backups")
	if err = os.MkdirAll(backupRoot, 0700); err != nil {
		return result, err
	}
	backupDir, err := os.MkdirTemp(backupRoot, now.Format("20060102-150405-"))
	if err != nil {
		return result, err
	}
	if nativeJSON != nil {
		if err = os.WriteFile(filepath.Join(backupDir, "native-profile-before.json"), nativeJSON, 0600); err != nil {
			return result, err
		}
	}
	backup := filepath.Join(backupDir, "sidelet.sqlite3")
	snapshot, err := storage.BackupLegacy(source, backup)
	if err != nil {
		return result, err
	}
	// Back up the exact preference bytes, then parse that backup so the value
	// being imported always matches its recoverable source file.
	if raw, e := os.ReadFile(filepath.Join(source, "settings.json")); e == nil {
		if err = os.WriteFile(filepath.Join(backupDir, "settings.json"), raw, 0600); err != nil {
			return result, err
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	preferences = settings.Open(backupDir)
	if preferences.LoadError != nil {
		return result, preferences.LoadError
	}
	v, err := convertLegacy(snapshot, preferences.Value)
	if err != nil {
		return result, err
	}
	if existing.ID != "" {
		v.ID = existing.ID
	}
	raw, err := os.ReadFile(backup)
	if err != nil {
		return result, err
	}
	hash := sha256.Sum256(raw)
	v.LegacyImport = &legacyImport{Source: source, Backup: backup, SHA256: hex.EncodeToString(hash[:]), ImportedAt: now.UnixMilli(), Stacks: snapshot.Stacks}
	for _, t := range snapshot.Todos {
		v.LegacyImport.OriginalIDs = append(v.LegacyImport.OriginalIDs, t.ID)
	}
	for _, name := range []string{backup, filepath.Join(backupDir, "settings.json"), filepath.Join(backupDir, "native-profile-before.json"), backupDir, backupRoot} {
		info, e := os.Stat(name)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return result, e
		}
		if info.IsDir() && runtime.GOOS == "windows" {
			continue
		}
		var file *os.File
		if info.IsDir() {
			file, e = os.Open(name)
		} else {
			file, e = os.OpenFile(name, os.O_RDWR, 0)
		}
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return result, e
		}
		e = file.Sync()
		file.Close()
		if e != nil {
			return result, e
		}
	}
	data, err = json.Marshal(v)
	if err != nil {
		return result, err
	}
	if err = db.Write("", data); err != nil {
		return result, err
	}
	return legacyImportResult{len(v.Tasks), backup, false}, nil
}
