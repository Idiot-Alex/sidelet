package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCompositionConfirmationDoesNotSubmitOrInsertNewline(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		t.Run(map[bool]string{false: "title", true: "note"}[multiline], func(t *testing.T) {
			value, submitted := "", 0
			u := ui.NewTester(func(c *ui.Context) {
				var field ui.Element
				if multiline {
					field = webTextArea(c, &value)
				} else {
					field = webTextInput(c, &value)
				}
				field.AutoFocus()
				if field.Submitted() {
					submitted++
				}
			}, 320, 160)
			u.SetFocused(true)
			u.Compose("中文", 2)
			u.Key(0, ui.KeyEnter)
			if submitted != 0 || value != "" {
				t.Fatalf("candidate confirmation changed draft: %q, submits=%d", value, submitted)
			}
			u.Type("中文")
			u.Key(0, ui.KeyEnter)
			if multiline {
				if value != "中文\n" || submitted != 0 {
					t.Fatalf("normal newline failed: %q, submits=%d", value, submitted)
				}
			} else if value != "中文" || submitted != 1 {
				t.Fatalf("normal submit failed: %q, submits=%d", value, submitted)
			}
		})
	}
}
