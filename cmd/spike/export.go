//go:build windows || darwin

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"sidelet/internal/exportdata"
)

type exportJob struct {
	request message
	count   int
	path    string
	err     error
}

// The queue owns activeExport. A single background operation waits for the
// native dialog and writes the captured snapshot; reminders keep running.
func (c *controller) startExport(m message) {
	if c.control == nil || m.Window != c.control || m.RequestID == "" {
		return
	}
	fail := func(err error) { c.finishExport(&exportJob{request: m, err: err}) }
	if c.store == nil {
		fail(fmt.Errorf("请在正常任务版本中导出数据。"))
		return
	}
	if err := exportdata.ValidateFormat(m.Format); err != nil {
		fail(err)
		return
	}
	if c.activeExport != nil {
		fail(fmt.Errorf("已有导出正在进行，请先完成或取消保存窗口。"))
		return
	}
	now := time.Now()
	state, err := c.store.Snapshot(now)
	if err != nil {
		fail(err)
		return
	}
	data, err := exportdata.Encode(state, m.Format, appVersion, now)
	if err != nil {
		fail(err)
		return
	}
	job := &exportJob{request: m, count: len(state.Todos)}
	c.activeExport = job
	dialog := c.app.Dialog.SaveFile().AttachToWindow(c.control).
		SetFilename("Sidelet-"+now.Format("20060102-150405")+"."+m.Format).
		SetMessage("导出全部任务，包含已完成与暂时隐藏的任务。").SetButtonText("导出").
		AddFilter(m.Format+" 文件", "*."+m.Format).HideExtension(false)
	go func() {
		job.path, job.err = dialog.PromptForSingleSelection()
		if job.err == nil && job.path != "" {
			job.err = exportdata.Save(job.path, m.Format, data, c.profileDirectory)
		}
		c.post(message{Type: "export-finished", Export: job})
	}()
}

func (c *controller) finishExport(job *exportJob) {
	result := map[string]any{"requestId": job.request.RequestID, "error": "", "cancelled": job.path == "" && job.err == nil}
	if job.err != nil {
		text := job.err.Error()
		if _, ok := job.err.(*os.PathError); ok {
			text = "无法写入所选位置，请检查文件夹权限和剩余空间后重试。"
		}
		result["error"] = "导出失败：" + text
		log.Printf("export failed format=%s: %v", job.request.Format, job.err)
	} else if job.path != "" {
		result["filename"] = filepath.Base(job.path)
		result["count"] = job.count
		log.Printf("export complete format=%s tasks=%d", job.request.Format, job.count)
	}
	application.InvokeSync(func() { job.request.Window.EmitEvent("todo:result", result) })
}
