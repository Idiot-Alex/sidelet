package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// An incomplete date is a draft, never a normalized time.Time. This keeps
// February 30 and nonexistent DST wall times visible until the user fixes them.
type deadlineSegments struct {
	parts           [5]string
	seen            string
	ready           bool
	typed           string
	typedAt         time.Time
	focused         int
	selectAll, open bool
}

var deadlineSegmentLabels = [5]string{"截止年份", "截止月份", "截止日期", "截止小时", "截止分钟"}
var deadlineSegmentMax = [5]int{9999, 12, 31, 23, 59}
var deadlineSegmentMin = [5]int{1, 1, 1, 0, 0}

func (v *nativeTasksView) syncDeadlineSegments() {
	s := &v.deadline
	if s.ready && s.seen == v.formDue {
		return
	}
	*s = deadlineSegments{seen: v.formDue, ready: true, focused: -1}
	if v.formDue == "" {
		return
	}
	parts := strings.FieldsFunc(v.formDue, func(r rune) bool { return strings.ContainsRune("/-T :年月日", r) })
	if len(parts) == 5 {
		copy(s.parts[:], parts)
	}
}

func (v *nativeTasksView) commitDeadlineSegments() {
	s := &v.deadline
	if s.parts == [5]string{} {
		v.formDue, v.formRemind = "", false
	} else {
		v.formDue = fmt.Sprintf("%s/%s/%s %s:%s", s.parts[0], s.parts[1], s.parts[2], s.parts[3], s.parts[4])
	}
	s.seen = v.formDue
}

func (v *nativeTasksView) webDeadlineSegments(c *ui.Context, field *ui.Element) {
	v.syncDeadlineSegments()
	s, t := &v.deadline, v.visual
	now := v.service.m.now()
	ghost := [5]int{now.Year(), int(now.Month()), now.Day(), now.Hour(), now.Minute()}
	var segments [5]*ui.Element
	anyFocused := false
	for i, label := range deadlineSegmentLabels {
		if i > 0 {
			separator := "/"
			if i == 3 {
				ui.Box(c).Width(5).Shrink(0).Role(ui.RoleNone)
			}
			if i == 4 {
				separator = ":"
			}
			if i != 3 {
				ui.Text(c, separator).Role(ui.RoleNone).FontSize(12).Height(16).FixedLineHeight(16)
			}
		}
		width := float32(16)
		if i == 0 {
			width = 30
		}
		seg := ui.Box(c).Key(label).Label(label).Role(ui.RoleStepper).Focusable().FocusRing(false).Height(18).Width(width).Shrink(0).Radius(2).AlignItems(ui.Center).Justify(ui.Center).TextCaret(ui.Rect{W: width, H: 18})
		n, _ := strconv.Atoi(s.parts[i])
		seg.Range(float64(deadlineSegmentMin[i]), float64(deadlineSegmentMax[i]), float64(n)).Step(1).Value(s.parts[i])
		segments[i] = seg
		if seg.Focused() {
			anyFocused = true
			field.BorderColor(t.Accent)
			seg.Background(t.Accent).TextColor(t.AccentText)
			if s.focused != i {
				s.focused, s.typed, s.selectAll = i, "", false
			}
		} else if s.parts[i] == "" {
			seg.TextColor(t.Subtle)
		}
		seg.Children(func() {
			text := s.parts[i]
			if text == "" {
				text = fmt.Sprintf("%02d", ghost[i])
			}
			ui.Text(c, text).FontSize(12).FontFeatures("tnum").Height(16).FixedLineHeight(16).NoWrap()
		})
	}
	if !anyFocused {
		s.focused, s.typed, s.selectAll = -1, "", false
	}
	for i, seg := range segments {
		move := func(next int) {
			if next >= 0 && next < len(segments) {
				segments[next].Focus()
				s.focused, s.typed, s.selectAll = next, "", false
			}
		}
		step := func(delta int) {
			n, err := strconv.Atoi(s.parts[i])
			if err != nil {
				n = ghost[i]
			} else {
				n += delta
			}
			lo, hi := deadlineSegmentMin[i], deadlineSegmentMax[i]
			if i == 2 {
				y, ey := strconv.Atoi(s.parts[0])
				m, em := strconv.Atoi(s.parts[1])
				if ey == nil && em == nil && y > 0 && m >= 1 && m <= 12 {
					hi = time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
				}
			}
			if n > hi {
				n = lo
			}
			if n < lo {
				n = hi
			}
			format := "%02d"
			if i == 0 {
				format = "%04d"
			}
			s.parts[i], s.typed = fmt.Sprintf(format, n), ""
			v.commitDeadlineSegments()
		}
		// Register shortcuts for accessibility's increment/decrement actions.
		if seg.Shortcut(0, ui.KeyUp) {
			step(1)
		}
		if seg.Shortcut(0, ui.KeyDown) {
			step(-1)
		}
		seg.HandleInput(func(ev ui.InputEvent) bool {
			switch ev.Kind {
			case ui.InputPointerDown:
				s.typed, s.selectAll = "", false
				return false // Preserve normal pointer focus and activation.
			case ui.InputCommand:
				if ev.Text == "copy" || ev.Text == "cut" {
					text := s.parts[i]
					if s.selectAll {
						text = v.formDue
					}
					c.WriteClipboard(text)
					if ev.Text == "copy" {
						return true
					}
				}
				if ev.Text == "paste" {
					text := strings.TrimSpace(c.ReadClipboard())
					if _, err := strconv.Atoi(text); err == nil && len(text) <= 4 {
						digits := 2
						if i == 0 {
							digits = 4
						}
						if len(text) <= digits {
							n, _ := strconv.Atoi(text)
							format := "%02d"
							if i == 0 {
								format = "%04d"
							}
							s.parts[i], s.typed, s.selectAll = fmt.Sprintf(format, n), "", false
							v.commitDeadlineSegments()
						}
					} else if at, err := parseDue(text, v.originalDue, v.originalDueText, now.Location()); err == nil && at > 0 {
						v.formDue = localDue(at, now.Location())
						v.syncDeadlineSegments()
					}
					return true
				}
				if ev.Text == "selectAll" {
					s.selectAll, s.typed = true, ""
					return true
				}
				if ev.Text != "delete" && ev.Text != "cut" {
					return false
				}
				fallthrough
			case ui.InputKeyDown:
				if ev.Kind == ui.InputKeyDown {
					// Key events can arrive before a render observes the new
					// focus. End the digit sequence on Tab immediately.
					if ev.Key == ui.KeyTab && (ev.Mods == 0 || ev.Mods == ui.Shift) {
						s.typed, s.selectAll = "", false
						return false
					}
					if ev.Mods == ui.Alt && ev.Key == ui.KeyDown {
						s.open = true
						return true
					}
					if ev.Mods != 0 {
						return false
					}
					switch ev.Key {
					case ui.KeyLeft:
						move(i - 1)
						return i > 0
					case ui.KeyRight:
						move(i + 1)
						return i < 4
					case ui.KeyBackspace, ui.KeyDelete:
					default:
						return false
					}
				}
				if s.selectAll {
					s.parts = [5]string{}
				} else {
					s.parts[i] = ""
				}
				s.selectAll, s.typed = false, ""
				v.commitDeadlineSegments()
				return true
			case ui.InputText:
				// Ignore composition and non-digits; committed ASCII digits are
				// handled once, without interpreting a partial year as a date.
				if ev.Text == "" || strings.IndexFunc(ev.Text, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
					return true
				}
				if c.Now().Sub(s.typedAt) > time.Second || s.focused != i {
					s.typed = ""
				}
				s.focused, s.selectAll, s.typedAt = i, false, c.Now()
				s.typed += ev.Text
				digits := 2
				if i == 0 {
					digits = 4
				}
				if len(s.typed) > digits {
					s.typed = ev.Text
				}
				if len(s.typed) > digits {
					return true
				}
				n, _ := strconv.Atoi(s.typed)
				format := "%02d"
				if i == 0 {
					format = "%04d"
				}
				s.parts[i] = fmt.Sprintf(format, n)
				v.commitDeadlineSegments()
				if len(s.typed) == digits {
					move(i + 1)
				}
				return true
			}
			return false
		})
	}
	field.DrawOver(func(p *ui.Painter, r ui.Rect) {
		for _, segment := range segments {
			if segment.FocusVisible() {
				p.Stroke(ui.Rect{X: r.X - 5, Y: r.Y - 5, W: r.W + 10, H: r.H + 10}, t.Focus, t.Theme.Radius+5, 2)
				break
			}
		}
	})
}
