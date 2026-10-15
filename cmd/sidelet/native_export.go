package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"sidelet/internal/exportdata"
	"sidelet/internal/todo"
)

// Owned by the service, so dialog results survive closing/reopening the view.
// State is accessed only on the serial UI executor; workers own immutable data.
type nativeExport struct {
	s            *TasksService
	busy, failed bool
	status       string
	notify       func()
	dialog       func(*mygo.Window, string, string) (string, error)
	save         func(string, string, []byte, string) error
}

func newNativeExport(s *TasksService, notify func()) *nativeExport {
	return &nativeExport{s: s, notify: notify, save: exportdata.Save, dialog: func(parent *mygo.Window, format, filename string) (string, error) {
		// A bare filename is resolved against the process directory by MyGo;
		// a Finder launch would otherwise default to the disk root.
		dir, err := mygo.App.Path(mygo.PathDownloads)
		if err != nil || dir == "" {
			dir, _ = os.UserHomeDir()
		}
		return mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: parent, Title: "导出任务", ButtonLabel: "导出", DefaultPath: filepath.Join(dir, filename),
			Message: "导出全部已保存任务，请选择资料目录以外的位置。", NameFieldLabel: "文件名：", CreateDirectories: true,
			Filters: []mygo.FileFilter{{Name: "Sidelet " + format, Extensions: []string{format}}}})
	}}
}

func (v *nativeTasksView) exportBusy() bool { return v.service.export != nil && v.service.export.busy }

// Use the last durable commit, including its hidden task slots. Never clean up
// or change the model to export, and never include editor/drag/quiet state.
func nativeExportSnapshot(m *model) todo.Snapshot {
	var v profileState
	if m.profile != nil {
		v = m.profile.saved
	} else {
		v = m.profileState("")
	}
	ordered := &model{Tasks: v.Tasks, Order: v.Order}
	density := "normal"
	if v.ItemHeight == 40 {
		density = "compact"
	}
	if v.ItemHeight == 56 {
		density = "relaxed"
	}
	out := todo.Snapshot{Todos: []todo.Todo{}, Stacks: []todo.EdgeStack{{ID: 1, Side: v.Side, Offset: v.Offset, Density: density}}}
	for position, i := range ordered.orderedIndices() {
		t := v.Tasks[i]
		if t.Deleted {
			continue
		}
		item := todo.Todo{ID: int64(i + 1), Title: t.Title, Description: t.Note, Priority: t.Priority, Temporary: t.Temporary,
			DueAt: t.DueAt, Remind: t.Remind, RemindAt: t.ReminderAt, ReminderSentAt: t.ReminderSentAt,
			Completed: t.Done, CompletedAt: t.CompletedAt, SnoozedUntil: t.SnoozedUntil, DisplayMode: "NONE"}
		if !t.Unpinned {
			item.DisplayMode, item.StackID, item.SortOrder = "EDGE", 1, position
		}
		out.Todos = append(out.Todos, item)
	}
	return out
}

func (e *nativeExport) start(format string, parent *mygo.Window) {
	var snapshot todo.Snapshot
	var at time.Time
	var profileDir string
	started := false
	e.s.run(func() {
		if e.busy {
			return
		}
		if err := exportdata.ValidateFormat(format); err != nil {
			e.failed, e.status = true, err.Error()
			if e.notify != nil {
				e.notify()
			}
			return
		}
		snapshot, at = nativeExportSnapshot(e.s.m), e.s.m.now()
		if e.s.m.profile != nil {
			profileDir = filepath.Dir(e.s.m.profile.path)
		}
		e.busy, e.failed, e.status, started = true, false, "请选择保存位置…", true
		if e.notify != nil {
			e.notify()
		}
	})
	if !started {
		return
	}
	go func() {
		filename := "Sidelet-" + at.Format("20060102-150405") + "." + format
		data, err := exportdata.Encode(snapshot, format, "0.0.1-mygo-lab", at)
		path := ""
		if err == nil {
			path, err = e.dialog(parent, format, filename)
		}
		if err == nil && path != "" {
			err = e.save(path, format, data, profileDir)
		}
		e.s.run(func() {
			e.busy, e.failed = false, err != nil
			switch {
			case err != nil:
				log.Printf("native export %s failed: %v", format, err)
				e.status = "导出失败：" + nativeExportError(err)
			case path == "":
				e.status = "已取消导出。"
			default:
				e.status = fmt.Sprintf("已导出 %d 个任务到 %s。", len(snapshot.Todos), filepath.Base(path))
			}
			if e.notify != nil {
				e.notify()
			}
		})
	}()
}

func nativeExportError(err error) string {
	var pathError *os.PathError
	var linkError *os.LinkError
	if errors.As(err, &pathError) || errors.As(err, &linkError) {
		return "无法写入所选位置，请检查文件夹权限和剩余空间后重试。"
	}
	return err.Error()
}
