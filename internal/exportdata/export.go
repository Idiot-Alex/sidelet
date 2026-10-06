// Package exportdata produces portable task files without changing user data.
package exportdata

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"sidelet/internal/todo"
)

const Version = 1

type Document struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	AppVersion string           `json:"appVersion"`
	ExportedAt string           `json:"exportedAt"`
	Todos      []todo.Todo      `json:"todos"`
	Stacks     []todo.EdgeStack `json:"stacks"`
}

func ValidateFormat(format string) error {
	if format != "json" && format != "csv" {
		return errors.New("请选择 JSON 或 CSV 导出。")
	}
	return nil
}

func Encode(state todo.Snapshot, format, appVersion string, now time.Time) ([]byte, error) {
	if err := ValidateFormat(format); err != nil {
		return nil, err
	}
	if format == "json" {
		doc := Document{Format: "sidelet", Version: Version, AppVersion: appVersion,
			ExportedAt: now.UTC().Format(time.RFC3339Nano),
			Todos:      append([]todo.Todo{}, state.Todos...), Stacks: append([]todo.EdgeStack{}, state.Stacks...)}
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	}
	// A UTF-8 BOM lets spreadsheet applications recognise Chinese text.
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	header := []string{"id", "title", "description", "completed", "priority", "temporary", "dueAt", "completedAt", "snoozedUntil", "remind", "remindAt", "reminderSentAt", "createdAt", "updatedAt", "displayMode", "stackId", "sortOrder", "displayId", "side", "offset", "density"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	stacks := make(map[int64]todo.EdgeStack, len(state.Stacks))
	for _, stack := range state.Stacks {
		stacks[stack.ID] = stack
	}
	for _, item := range state.Todos {
		layout := []string{"", "", "", ""}
		if stack, ok := stacks[item.StackID]; ok {
			layout = []string{cell(stack.DisplayID), stack.Side, strconv.FormatFloat(stack.Offset, 'g', -1, 64), stack.Density}
		}
		row := []string{integer(item.ID), cell(item.Title), cell(item.Description), strconv.FormatBool(item.Completed), strconv.Itoa(item.Priority), strconv.FormatBool(item.Temporary),
			timestamp(item.DueAt), timestamp(item.CompletedAt), timestamp(item.SnoozedUntil), strconv.FormatBool(item.Remind), timestamp(item.RemindAt), timestamp(item.ReminderSentAt),
			timestamp(item.CreatedAt), timestamp(item.UpdatedAt), item.DisplayMode, integer(item.StackID), strconv.Itoa(item.SortOrder)}
		if err := w.Write(append(row, layout...)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func integer(value int64) string { return strconv.FormatInt(value, 10) }
func timestamp(value int64) string {
	if value == 0 {
		return ""
	}
	return time.UnixMilli(value).UTC().Format(time.RFC3339Nano)
}

// JSON retains original text; CSV prefixes risky cells with an apostrophe to
// keep spreadsheet programs from evaluating task text as a formula.
func cell(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if (len(value) > 0 && strings.ContainsRune("\t\r\n", rune(value[0]))) || (len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + value
	}
	return value
}

// Save stages the complete file beside its destination before replacing it.
// Failed writes/renames leave an existing export intact and remove the staging
// file. Exporting into the live profile directory is never allowed.
func Save(path, format string, data []byte, profileDirectory string) error {
	if err := ValidateFormat(format); err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		return fmt.Errorf("文件名需要以 .%s 结尾，请重新选择保存位置。", format)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return err
	}
	if profileDirectory != "" {
		profile, err := os.Stat(profileDirectory)
		if err != nil {
			return err
		}
		// Compare directory identity, also on case-insensitive filesystems and
		// through symbolic links. String prefixes do not protect those aliases.
		for current := parent; ; {
			info, err := os.Stat(current)
			if err != nil {
				return err
			}
			if os.SameFile(profile, info) {
				return errors.New("请选择 Sidelet 资料目录以外的位置，避免覆盖正在使用的数据。")
			}
			next := filepath.Dir(current)
			if next == current {
				break
			}
			current = next
		}
	}
	file, err := os.CreateTemp(parent, ".sidelet-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(parent, filepath.Base(path)))
}
