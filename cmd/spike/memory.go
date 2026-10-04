//go:build windows || darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"sidelet/internal/diagnostics"
	"sidelet/internal/platform"
)

// No polling, GC, registry or extra frontend subscriptions without -memory-dir.
func (c *controller) watchMemoryRequests() {
	if c.memoryDir == "" {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			request := filepath.Join(c.memoryDir, "request.txt")
			data, err := os.ReadFile(request)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				log.Printf("memory request: %v", err)
				continue
			}
			if err := os.Remove(request); err != nil {
				log.Printf("memory request: %v", err)
				continue
			}
			label := strings.TrimSpace(string(data))
			path, err := diagnostics.Reserve(c.memoryDir, label)
			if err != nil {
				log.Printf("memory request: %v", err)
				continue
			}
			// The worker waits for native sampling; profiles and file writes follow
			// outside InvokeSync. Frontend replies carry the same label.
			var native json.RawMessage
			application.InvokeSync(func() {
				c.memoryLabel = label
				c.memoryViews = map[string]bool{}
				native = platform.MemoryDiagnostic()
				c.app.Event.Emit("spike:memory-request", label)
			})
			if err := diagnostics.WriteJSON(filepath.Join(path, "native.json"), native); err != nil {
				log.Print(err)
			}
			if err := diagnostics.Capture(path); err != nil {
				log.Printf("memory capture: %v", err)
				continue
			}
			log.Printf("memory captured label=%s path=%s", label, path)
		}
	}
}

func (c *controller) memoryView(m message) error {
	if c.memoryDir == "" || m.Window == nil || m.Label != c.memoryLabel {
		return nil
	}
	name := m.Window.Name()
	if name != "control" && c.find(m.Window) == nil {
		return nil
	}
	if c.memoryViews[name] {
		return nil
	}
	c.memoryViews[name] = true
	if !json.Valid(m.Metric) {
		return fmt.Errorf("invalid frontend memory sample")
	}
	return diagnostics.WriteJSON(filepath.Join(c.memoryDir, m.Label, name+".json"), map[string]any{
		"at": time.Now().UTC(), "window": name, "label": m.Label, "metrics": m.Metric,
	})
}
