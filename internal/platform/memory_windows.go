//go:build windows

package platform

import "encoding/json"

func EnableMemoryDiagnostics() {}
func MemoryDiagnostic() json.RawMessage {
	return json.RawMessage(`{"supported":false,"platform":"windows"}`)
}
func (w *Window) EnableInteractionTest() error { return nil }
func FocusDiagnostic() string                  { return `{"platform":"windows","supported":false}` }
func (token FocusToken) Diagnostic() string    { return `{"platform":"windows","supported":false}` }
