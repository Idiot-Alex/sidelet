package main

import (
	"errors"
	"fmt"

	"sidelet/internal/settings"
)

func densityHeight(density string) int {
	switch density {
	case "compact":
		return 40
	case "relaxed":
		return 56
	default:
		return 44
	}
}

// Quiet is a presentation state, as in the formal app. It does not stop
// reminder delivery, mutate tasks or enter the durable profile.
func (s *TasksService) SetQuiet(enabled bool) (out taskSnapshot, err error) {
	s.run(func() {
		m := s.m
		m.cancelDrag()
		m.close()
		m.Arranging, m.Quiet = false, enabled
		if s.changed != nil {
			s.changed("native-main-quiet")
		}
		out = s.snapshot()
	})
	return
}

// Native Dock policy is applied before saving, then compensated if the file
// commit fails. The saved preference never advertises an unapplied policy.
func (s *TasksService) SavePreferences(value settings.Value) (out taskSnapshot, err error) {
	s.run(func() {
		if err = value.Validate(); err != nil {
			return
		}
		if value.Startup.Enabled {
			err = errors.New("实验应用尚未接入登录启动。")
			return
		}
		if s.m.Dragging {
			err = errors.New("请先结束侧栏拖动。")
			return
		}
		previous := s.m.Preferences.Appearance.ShowDockIcon
		changeDock := previous != value.Appearance.ShowDockIcon
		if changeDock {
			if s.applyDock == nil {
				err = errors.New("当前平台暂不支持 Dock 显示设置。")
				return
			}
			if err = s.applyDock(value.Appearance.ShowDockIcon); err != nil {
				if rollback := s.applyDock(previous); rollback != nil {
					err = fmt.Errorf("无法更改或恢复 Dock 显示，请重试：%v（%v）", err, rollback)
				}
				return
			}
		}
		out, err = s.mutateOnMain("preferences", func() error {
			s.m.Preferences = value
			s.m.UITheme = value.Appearance.Theme
			return nil
		})
		if err != nil && changeDock {
			if rollback := s.applyDock(previous); rollback != nil {
				err = fmt.Errorf("设置保存失败，恢复 Dock 显示也失败；请重试：%v（%v）", err, rollback)
				s.m.storageError = err.Error()
			}
		}
	})
	return
}

func startupMainVisible(m *model, forceShow, forceHidden bool) bool {
	if forceHidden {
		return false
	}
	return forceShow || m.profile == nil || m.profile.fresh || m.Preferences.Startup.ShowMainWindow
}
