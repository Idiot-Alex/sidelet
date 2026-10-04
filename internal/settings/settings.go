// Package settings owns app preferences. Task and existing Stack state stay in SQLite.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Edge struct {
	DefaultSide    string `json:"defaultSide"`
	DefaultDensity string `json:"defaultDensity"`
}
type Startup struct {
	Enabled        bool `json:"enabled"`
	ShowMainWindow bool `json:"showMainWindow"`
}
type Appearance struct {
	Theme string `json:"theme"`
}
type Value struct {
	Version    int        `json:"version"`
	Edge       Edge       `json:"edge"`
	Startup    Startup    `json:"startup"`
	Appearance Appearance `json:"appearance"`
}

func Defaults() Value {
	return Value{Version: 1, Edge: Edge{"right", "normal"}, Appearance: Appearance{Theme: "mac"}}
}
func (v Value) Validate() error {
	if v.Version != 1 {
		return errors.New("设置文件版本不受支持。")
	}
	switch v.Appearance.Theme {
	case "mac", "paper", "graphite":
	default:
		return errors.New("请选择有效的界面主题。")
	}
	if v.Edge.DefaultSide != "left" && v.Edge.DefaultSide != "right" {
		return errors.New("请选择有效的默认边缘。")
	}
	switch v.Edge.DefaultDensity {
	case "compact", "normal", "relaxed":
	default:
		return errors.New("请选择有效的标签密度。")
	}
	return nil
}

// Store is accessed by the controller's serial worker; Value is published after commit.
type Store struct {
	path      string
	Value     Value
	Exists    bool
	LoadError error
	replace   func(string, string) error
}

func Open(directory string) *Store {
	s := &Store{path: filepath.Join(directory, "settings.json"), Value: Defaults(), replace: os.Rename}
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s
	}
	s.Exists = true
	if err == nil {
		// Preserve partial older files through defaults, but never silently discard unknown fields.
		v := Defaults()
		var fields map[string]json.RawMessage
		err = json.Unmarshal(data, &fields)
		if err == nil && fields == nil {
			err = errors.New("设置文件必须为 JSON 对象。")
		}
		if err == nil {
			for k := range fields {
				if k != "version" && k != "edge" && k != "startup" && k != "appearance" {
					err = fmt.Errorf("未知设置字段：%s", k)
					break
				}
			}
		}
		if err == nil {
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&v)
		}
		if err == nil {
			err = v.Validate()
		}
		if err == nil {
			s.Value = v
		}
	}
	s.LoadError = err
	return s
}
func (s *Store) Save(v Value) error {
	if s.LoadError != nil {
		return errors.New("设置文件无法读取，已保留原文件。请检查 settings.json 后重启。")
	}
	if err := v.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = s.replace(name, s.path); err != nil {
		return err
	}
	s.Value, s.Exists = v, true
	return nil
}

// ChangeLogin compensates a failed file commit and reports any failed compensation.
// System status remains authoritative; startup never re-registers on its own.
func (s *Store) ChangeLogin(enabled, wasRegistered bool, apply func(bool) error) error {
	if s.LoadError != nil {
		return errors.New("设置文件无法读取，无法更改登录启动。")
	}
	if err := apply(enabled); err != nil {
		return err
	}
	v := s.Value
	v.Startup.Enabled = enabled
	if err := s.Save(v); err != nil {
		if rollback := apply(wasRegistered); rollback != nil {
			return fmt.Errorf("设置保存失败，恢复系统登录项也失败；请检查系统登录项：%v（%v）", err, rollback)
		}
		return fmt.Errorf("设置保存失败，已恢复原登录状态：%w", err)
	}
	return nil
}

// UserMessage keeps file-system diagnostics in logs, with actionable UI copy.
func UserMessage(err error) string {
	if os.IsPermission(err) {
		return "无法保存设置，请检查资料目录的写入权限。原偏好已保留。"
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return "无法写入设置文件，请检查磁盘空间和资料目录后重试。原偏好已保留。"
	}
	return err.Error()
}
