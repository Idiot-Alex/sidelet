package spike

import "time"

// QuickSession joins pointer presence across two separate native windows.
// Revision invalidates delayed close messages after re-entry or reopening.
type QuickSession struct {
	Source          string
	Card            string
	TodoID          int64
	Open            bool
	Revision        uint64
	RequestRevision uint64
	CloseAt         time.Time
	sourceInside    bool
	quickInside     bool
}

func (s *QuickSession) Begin(source, card string, todoID int64) {
	s.Revision++
	s.RequestRevision++
	s.Source, s.Card, s.TodoID, s.Open = source, card, todoID, true
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
	case s.Card:
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
	s.RequestRevision++
	s.Open = false
	s.TodoID = 0
	s.CloseAt = time.Time{}
}

// Pointer changes invalidate close timers, but must not invalidate the renderer
// replying to the same opening request. A cancelled/replaced request must never
// become visible when its asynchronous WebView load finally completes.
func (s *QuickSession) CanPresent(request uint64, now time.Time, active bool) bool {
	return s.Open && request == s.RequestRevision && (active || s.CloseAt.IsZero() || now.Before(s.CloseAt))
}
