package main

import "log"

const nativeQuickAccelerator = "Ctrl+Shift+Space"
const nativeQuickDiagnosticAccelerator = "Ctrl+Alt+Shift+F19"

type nativeQuickShortcut struct {
	registered bool
	error      string
	stop       func()
	generation uint64
	diagnostic bool
	retry      func()
}

func (s *nativeQuickShortcut) register(register func(func()) (func(), error), open func()) {
	if s.registered {
		return
	}
	s.generation++
	generation := s.generation
	stop, err := register(func() {
		if s.registered && generation == s.generation {
			open()
		}
	})
	if err != nil {
		s.error = "快捷键注册失败，可能已被其他应用占用。仍可从菜单栏或任务窗口快速添加；请退出其他 Sidelet 实例或解除占用，再点击“重试快捷键”。"
		log.Printf("native quick-add shortcut failed: %v", err)
		return
	}
	s.registered, s.stop, s.error = true, stop, ""
	name := nativeQuickAccelerator
	if s.diagnostic {
		name = nativeQuickDiagnosticAccelerator
	}
	log.Printf("native quick-add shortcut registered shortcut=%s diagnostic=%t", name, s.diagnostic)
}
func (s *nativeQuickShortcut) close() {
	s.registered = false
	s.generation++
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
}
