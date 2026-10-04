package spike

import "time"

// QuickSession joins pointer presence across two separate native windows.
// Revision invalidates delayed close messages after re-entry or reopening.
type QuickSession struct {
	Source       string
	TodoID       int64
	Open         bool
	Revision     uint64
	CloseAt      time.Time
	sourceInside bool
	quickInside  bool
}

func (s *QuickSession) Begin(source string, todoID int64) {
	s.Revision++
	s.Source, s.TodoID, s.Open = source, todoID, true
	s.sourceInside, s.quickInside = true, false
	s.CloseAt = time.Time{}
}

func (s *QuickSession) Presence(window string, inside bool, now time.Time) {
	if !s.Open {
		return
	}
	switch window {
	case s.Source:
		s.sourceInside = inside
	case "quick":
		s.quickInside = inside
	default:
		return
	}
	s.Revision++
	if s.sourceInside || s.quickInside {
		s.CloseAt = time.Time{}
	} else {
		s.CloseAt = now.Add(500 * time.Millisecond)
	}
}

func (s *QuickSession) Expired(revision uint64, now time.Time) bool {
	return s.Open && s.Revision == revision && !s.CloseAt.IsZero() && !now.Before(s.CloseAt)
}

func (s *QuickSession) Close() {
	s.Revision++
	s.Open = false
	s.TodoID = 0
	s.CloseAt = time.Time{}
}
