package quickadd

import "time"

// Session is owned by the UI thread. Revision rejects delayed input from a
// previous opening; Reference keeps date preview and save identical at midnight.
type Session struct {
	Open         bool      `json:"open"`
	Saving       bool      `json:"saving"`
	Revision     uint64    `json:"revision"`
	ResetVersion uint64    `json:"resetVersion"`
	Reference    time.Time `json:"-"`
}

func (s *Session) Begin(now time.Time) bool {
	if s.Saving || s.Open {
		return false
	}
	s.Open = true
	s.Revision++
	s.Reference = now
	return true
}
func (s *Session) StartSave(revision uint64) bool {
	if !s.Open || s.Saving || revision != s.Revision {
		return false
	}
	s.Saving = true
	return true
}
func (s *Session) Cancel() bool {
	if !s.Open || s.Saving {
		return false
	}
	s.Open = false
	s.ResetVersion++
	return true
}

func (s *Session) Blur() { s.Open = false }
func (s *Session) FinishSave(success bool) bool {
	wasOpen := s.Open
	s.Saving = false
	if success {
		s.Open = false
		s.ResetVersion++
	}
	return success && wasOpen
}
