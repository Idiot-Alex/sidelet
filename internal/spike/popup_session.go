package spike

// PopupSession owns the physical window shared by the two mutually exclusive
// popup views. Every presentation gets a new generation, including same-view
// reopens, so delayed IPC cannot act on the next presentation.
type PopupSession struct {
	View      string
	Revision  uint64
	Open      bool
	Preparing bool
}

func (s *PopupSession) Prepare(view string) uint64 {
	s.View = view
	s.Revision++
	s.Open, s.Preparing = true, true
	return s.Revision
}

func (s *PopupSession) Accept(view string, revision uint64) bool {
	return s.Open && s.View == view && s.Revision == revision
}

func (s *PopupSession) Rendered(view string, revision uint64) bool {
	if !s.Preparing || !s.Accept(view, revision) {
		return false
	}
	s.Preparing = false
	return true
}

func (s *PopupSession) Close(view string) {
	if !s.Open || s.View != view {
		return
	}
	s.Open, s.Preparing = false, false
	s.Revision++
}
