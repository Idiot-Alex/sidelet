//go:build darwin

package main

import "testing"

func TestNativeQuickHotkeyRejectsUnrelatedAndMalformedEvents(t *testing.T) {
	for _, item := range []struct {
		status   int32
		id       carbonQuickID
		accepted bool
	}{
		{0, carbonQuickID{quickCarbonSignature, 1}, true},
		{-1, carbonQuickID{quickCarbonSignature, 1}, false},
		{0, carbonQuickID{quickCarbonSignature, 2}, false},
		{0, carbonQuickID{0x4d79476f, 1}, false},
		{0, carbonQuickID{0x534c6574, 2}, false},
	} {
		if nativeQuickHotkeyMatches(item.status, item.id) != item.accepted {
			t.Fatal("foreign shortcut reached Quick Add", item)
		}
	}
}
