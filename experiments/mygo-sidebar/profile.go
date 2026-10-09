package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"sidelet/internal/settings"
	"sidelet/internal/todo"
)

// Persist only committed domain state. Window coordinates, input drafts,
// hover, drag previews and arrange mode never enter this file.
type profileState struct {
	Preferences        settings.Value `json:"preferences"`
	Schema             int            `json:"schema"`
	ID                 string         `json:"profileId"`
	Tasks              []task         `json:"tasks"`
	Order              []int          `json:"order"`
	Theme              string         `json:"theme"`
	Side               string         `json:"side"`
	Offset             float64        `json:"offset"`
	ItemHeight         int            `json:"itemHeight"`
	UndoID             int            `json:"undoId"`
	UndoUntil          int64          `json:"undoUntil"`
	ReminderGeneration int64          `json:"reminderGeneration"`
}

type profile struct {
	fresh bool
	path  string
	lock  *os.File
	saved profileState
	write func(string, []byte) error
}

func (m *model) profileState(id string) profileState {
	return profileState{m.Preferences, 2, id, slices.Clone(m.Tasks), slices.Clone(m.Order), m.UITheme, m.Side, m.Offset, m.itemHeight(), m.UndoID, m.UndoUntil, m.ReminderGeneration}
}

func validateProfile(v profileState) error {
	if v.Schema != 1 && v.Schema != 2 {
		return errors.New("资料版本不受支持，请使用对应版本的应用。")
	}
	if v.Schema == 2 {
		if err := v.Preferences.Validate(); err != nil {
			return err
		}
		if v.Preferences.Appearance.Theme != v.Theme {
			return errors.New("资料中的偏好状态无效。")
		}
	}
	if b, err := hex.DecodeString(v.ID); err != nil || len(b) != 16 {
		return errors.New("资料标识无效。")
	}
	if productionDesign.Themes[v.Theme] == nil || (v.Side != "left" && v.Side != "right") || math.IsNaN(v.Offset) || math.IsInf(v.Offset, 0) || v.Offset < 0 || v.Offset > 1 || (v.ItemHeight != 40 && v.ItemHeight != 44 && v.ItemHeight != 56) {
		return errors.New("资料中的主题或布局无效。")
	}
	if v.ReminderGeneration < 0 || v.UndoID < 0 || v.UndoID > len(v.Tasks) || v.UndoUntil < 0 {
		return errors.New("资料中的任务状态无效。")
	}
	seen := map[int]bool{}
	for _, id := range v.Order {
		if id < 1 || id > len(v.Tasks) || seen[id] {
			return errors.New("资料中的任务顺序无效。")
		}
		seen[id] = true
	}
	reminders := map[int64]bool{}
	for _, t := range v.Tasks {
		if t.Version == 0 {
			return errors.New("资料中的任务版本无效。")
		}
		if t.Deleted {
			continue
		}
		if _, err := validTitleLimit(t.Title, 500); err != nil {
			return err
		}
		if utf8.RuneCountInString(t.Note) > 16000 {
			return errors.New("资料中的备注过长。")
		}
		if !validPriority(t.Priority) || t.SnoozedUntil < 0 || t.CompletedAt < 0 || t.ReminderSentAt < 0 || t.ReminderID < 0 || t.ReminderID > v.ReminderGeneration || t.ReminderAt < 0 {
			return errors.New("资料中的任务字段无效。")
		}
		if err := todo.ValidateDeadline(t.DueAt, t.Remind); err != nil {
			return err
		}
		if t.Remind && t.DueAt > 0 {
			if t.ReminderID == 0 || t.ReminderAt != todo.EffectiveReminderAt(t.DueAt, t.SnoozedUntil) {
				return errors.New("资料中的提醒计划无效。")
			}
		} else if t.ReminderID != 0 || t.ReminderAt != 0 || t.ReminderSentAt != 0 {
			return errors.New("资料中的提醒回执无效。")
		}
		if t.ReminderID > 0 {
			if reminders[t.ReminderID] {
				return errors.New("资料中的提醒编号重复。")
			}
			reminders[t.ReminderID] = true
		}
	}
	if v.UndoID > 0 && (v.Tasks[v.UndoID-1].Deleted || !v.Tasks[v.UndoID-1].Done) {
		return errors.New("资料中的撤销对象无效。")
	}
	return nil
}

func openProfile(dir string, m *model) (*profile, error) {
	before := m.clone()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "profile.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = lockProfile(f); err != nil {
		f.Close()
		return nil, errors.New("这个资料目录已被其他 Sidelet MyGo Lab 进程使用，或无法锁定。")
	}
	p := &profile{path: filepath.Join(dir, "profile.json"), lock: f, write: atomicProfileWrite}
	ok := false
	defer func() {
		if !ok {
			p.close()
			*m = before
		}
	}()
	info, err := os.Lstat(p.path)
	if errors.Is(err, os.ErrNotExist) {
		id := make([]byte, 16)
		if _, err = rand.Read(id); err != nil {
			return nil, err
		}
		p.fresh = true
		m.Preferences.Appearance.Theme = m.UITheme
		m.Side, m.ItemHeight = m.Preferences.Edge.DefaultSide, densityHeight(m.Preferences.Edge.DefaultDensity)
		m.Tasks, m.Order = []task{}, nil
		p.saved.ID = hex.EncodeToString(id)
		if err = p.save(m); err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return nil, errors.New("资料文件类型或大小无效。")
		}
		f, err := os.Open(p.path)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(io.LimitReader(f, 64<<20))
		dec.DisallowUnknownFields()
		var state profileState
		err = dec.Decode(&state)
		if err == nil {
			var extra any
			if e := dec.Decode(&extra); e != io.EOF {
				err = errors.New("资料文件包含多余内容。")
			}
		}
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("无法读取资料；原文件未修改：%w", err)
		}
		if err = validateProfile(state); err != nil {
			return nil, err
		}
		p.saved = state
		m.Preferences = state.Preferences
		if state.Schema == 1 {
			m.Preferences = settings.Defaults()
			m.Preferences.Appearance.Theme = state.Theme
			m.Preferences.Startup.ShowMainWindow = true // Keep the previous prototype startup behavior.
		}
		m.Tasks, m.Order = slices.Clone(state.Tasks), slices.Clone(state.Order)
		m.UITheme, m.Side, m.Offset, m.ItemHeight = state.Theme, state.Side, state.Offset, state.ItemHeight
		m.UndoID, m.UndoUntil, m.ReminderGeneration = state.UndoID, state.UndoUntil, state.ReminderGeneration
	}
	m.profile = p
	if err = m.commit(func() error { return nil }); err != nil {
		m.profile = nil
		return nil, err
	}
	ok = true
	return p, nil
}

func (p *profile) save(m *model) error {
	if p.lock == nil {
		return errors.New("资料已关闭。")
	}
	next := m.profileState(p.saved.ID)
	if reflect.DeepEqual(next, p.saved) {
		return nil
	}
	if err := validateProfile(next); err != nil {
		return err
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = p.write(p.path, append(data, '\n')); err != nil {
		return err
	}
	p.saved = next
	return nil
}

func atomicProfileWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".profile-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	// The rename is the commit point. A directory-sync failure after this point
	// cannot roll the UI back to a state different from the committed file.
	if dir, e := os.Open(filepath.Dir(path)); e == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (p *profile) close() {
	if p != nil && p.lock != nil {
		_ = unlockProfile(p.lock)
		_ = p.lock.Close()
		p.lock = nil
	}
}

func profileDirectory(explicit, output string) (string, error) {
	if explicit == "" {
		if output != "" {
			explicit = filepath.Join(output, "profile")
		} else {
			root, err := os.UserConfigDir()
			if err != nil {
				return "", err
			}
			explicit = filepath.Join(root, "SideletMyGoLab")
		}
	}
	abs, err := filepath.Abs(explicit)
	if err != nil {
		return "", err
	}
	if candidate, e := filepath.EvalSymlinks(abs); e == nil {
		abs = candidate
	}
	if _, e := os.Lstat(filepath.Join(abs, "sidelet.sqlite3")); e == nil {
		return "", errors.New("请为 MyGo 实验选择独立资料目录。")
	}
	// The formal app's folder is not an experiment destination.
	if root, e := os.UserConfigDir(); e == nil {
		formal := filepath.Join(root, "Sidelet")
		if real, e := filepath.EvalSymlinks(formal); e == nil {
			formal = real
		}
		rel, e := filepath.Rel(strings.ToLower(formal), strings.ToLower(abs))
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("请为 MyGo 实验选择独立资料目录。")
		}
	}
	return abs, nil
}

func (m *model) clone() model {
	copy := *m
	copy.Tasks, copy.Order = slices.Clone(m.Tasks), slices.Clone(m.Order)
	return copy
}

func (m *model) commit(fn func() error) error {
	before := m.clone()
	if err := fn(); err != nil {
		*m = before
		return err
	}
	m.cleanup()
	if m.profile != nil {
		if err := m.prepareReminders(); err != nil {
			*m = before
			return err
		}
		changed := !reflect.DeepEqual(m.profileState(m.profile.saved.ID), m.profile.saved)
		if err := m.profile.save(m); err != nil {
			*m = before
			m.storageError = "保存失败，修改未生效；请检查资料目录后重试。"
			log.Printf("profile save failed: %v", err)
			return errors.New(m.storageError)
		}
		if changed {
			m.storageError = ""
		}
	} else {
		m.storageError = ""
	}
	return nil
}

func (m *model) prepareReminders() error {
	for i := range m.Tasks {
		t := &m.Tasks[i]
		if t.Deleted || !t.Remind || t.DueAt == 0 {
			t.ReminderID, t.ReminderAt, t.ReminderSentAt = 0, 0, 0
			continue
		}
		at := todo.EffectiveReminderAt(t.DueAt, t.SnoozedUntil)
		if t.ReminderID == 0 || t.ReminderAt != at {
			if m.ReminderGeneration == math.MaxInt64 {
				return errors.New("提醒编号已耗尽。")
			}
			m.ReminderGeneration++
			t.ReminderID, t.ReminderAt, t.ReminderSentAt = m.ReminderGeneration, at, 0
		}
	}
	return nil
}

func (m *model) persistenceHint() string {
	if m.profile != nil {
		return "自动保存在本机 · MyGo 独立资料"
	}
	return "独立原型 · 合成任务仅在本次运行中保留"
}
