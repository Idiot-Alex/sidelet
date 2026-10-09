package main

import (
	"errors"
	"fmt"
	"log"
)

type loginBackend interface {
	Status() (string, error)
	Set(bool) error
	OpenSettings() error
}

// System state is separate from the durable preference: refresh never registers
// an item, nor overwrites the last committed profile after an external change.
type nativeLogin struct {
	backend loginBackend
	status  string
	error   string
}

func loginRegistered(status string) bool {
	return status == "enabled" || status == "requiresApproval"
}

func loginLabel(status string) string {
	switch status {
	case "enabled":
		return "已开启"
	case "notRegistered":
		return "未开启"
	case "requiresApproval":
		return "等待系统批准"
	case "notFound":
		return "系统未找到此应用"
	case "unsupported":
		return "当前平台暂不支持"
	default:
		return "暂时无法读取"
	}
}

func (l *nativeLogin) refresh() error {
	status, err := l.backend.Status()
	if err != nil {
		log.Printf("login status: %v", err)
		l.status, l.error = "unknown", "无法读取系统登录项，请检查状态后重试。"
		return errors.New(l.error)
	}
	switch status {
	case "enabled", "notRegistered", "requiresApproval", "notFound", "unsupported":
		l.status, l.error = status, ""
		return nil
	default:
		l.status, l.error = "unknown", "系统返回了未知登录状态，请重试。"
		return errors.New(l.error)
	}
}

func (s *TasksService) CheckLogin() (err error) {
	s.run(func() {
		if s.login == nil {
			err = errors.New("当前平台暂不支持登录启动。")
			return
		}
		err = s.login.refresh()
		if s.changed != nil {
			s.changed("login-status")
		}
	})
	return
}

func (s *TasksService) ChangeLogin(enabled bool) (err error) {
	s.run(func() {
		if s.m.Dragging {
			err = errors.New("请先结束侧栏拖动。")
			return
		}
		l := s.login
		if l == nil {
			err = errors.New("当前平台暂不支持登录启动。")
			return
		}
		defer func() {
			if err != nil {
				l.error = err.Error()
			}
			if s.changed != nil {
				s.changed("login-status")
			}
		}()
		if err = l.refresh(); err != nil {
			return
		}
		if l.status == "unsupported" {
			err = fmt.Errorf("%s，请使用已构建的 macOS 实验应用。", loginLabel(l.status))
			return
		}
		previous := loginRegistered(l.status)
		changed := enabled != previous
		// Compensate even when Set reports failure: native APIs may have changed
		// registration before an error. Always re-read, including failed rollback.
		rollback := func(cause error) error {
			log.Printf("login change failed: %v", cause)
			var restore error
			if changed {
				restore = l.backend.Set(previous)
			}
			check := l.refresh()
			if restore != nil || check != nil || loginRegistered(l.status) != previous {
				log.Printf("login rollback: restore=%v check=%v status=%s", restore, check, l.status)
				return errors.New("登录启动设置失败，原系统状态未能确认恢复；请检查系统登录项。")
			}
			return errors.New("登录启动设置失败，已保留原偏好和系统登录状态；请重试。")
		}
		if changed {
			if cause := l.backend.Set(enabled); cause != nil {
				err = rollback(cause)
				return
			}
		}
		if cause := l.refresh(); cause != nil || loginRegistered(l.status) != enabled {
			err = rollback(errors.New("无法确认请求的系统登录状态"))
			return
		}
		if s.m.Preferences.Startup.Enabled != enabled {
			_, cause := s.mutateOnMain("login-preference", func() error {
				s.m.Preferences.Startup.Enabled = enabled
				return nil
			})
			if cause != nil {
				err = rollback(cause)
				return
			}
		}
	})
	return
}

func (s *TasksService) OpenLoginSettings() (err error) {
	s.run(func() {
		if s.login == nil {
			err = errors.New("当前平台暂不支持登录启动。")
			return
		}
		if cause := s.login.backend.OpenSettings(); cause != nil {
			log.Printf("open login settings: %v", cause)
			err = errors.New("无法打开系统登录项，请在系统设置中手动打开。")
		}
	})
	return
}
