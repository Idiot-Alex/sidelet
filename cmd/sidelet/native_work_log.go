package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type workLog struct {
	day           time.Time
	done, planned []task
}

func generateWorkLog(tasks []task, now time.Time) workLog {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow, after := day.AddDate(0, 0, 1), day.AddDate(0, 0, 2)
	log := workLog{day: day}
	for _, item := range tasks {
		if item.Deleted {
			continue
		}
		if item.Done && item.CompletedAt >= day.UnixMilli() && item.CompletedAt < tomorrow.UnixMilli() && item.CompletedAt > 0 {
			log.done = append(log.done, item)
		}
		if !item.Done && item.DueAt >= tomorrow.UnixMilli() && item.DueAt < after.UnixMilli() {
			log.planned = append(log.planned, item)
		}
	}
	sort.SliceStable(log.done, func(i, j int) bool { return log.done[i].CompletedAt < log.done[j].CompletedAt })
	sort.SliceStable(log.planned, func(i, j int) bool {
		if log.planned[i].DueAt == log.planned[j].DueAt {
			return log.planned[i].Priority > log.planned[j].Priority
		}
		return log.planned[i].DueAt < log.planned[j].DueAt
	})
	return log
}

func (l workLog) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "工作日志 · %s\n\n今日完成（%d 项）\n", l.day.Format("2006年1月2日"), len(l.done))
	write := func(items []task, completed bool) {
		for i, item := range items {
			at, label := item.DueAt, "预计"
			if completed {
				at, label = item.CompletedAt, "完成于"
			}
			fmt.Fprintf(&b, "%d. %s（%s %s）\n", i+1, strings.Join(strings.Fields(item.Title), " "), label, time.UnixMilli(at).In(l.day.Location()).Format("15:04"))
		}
	}
	if len(l.done) == 0 {
		b.WriteString("暂无已完成任务\n")
	}
	write(l.done, true)
	fmt.Fprintf(&b, "\n明日计划 · %s（%d 项）\n", l.day.AddDate(0, 0, 1).Format("1月2日"), len(l.planned))
	if len(l.planned) == 0 {
		b.WriteString("暂无预计明天完成的任务\n")
	}
	write(l.planned, false)
	return strings.TrimSpace(b.String())
}

func (v *nativeTasksView) webWorkLog(c *ui.Context) {
	now := v.service.m.now()
	log := generateWorkLog(v.service.m.Tasks, now)
	text := log.text()
	t := v.visual
	// Refresh an open log at local midnight, including DST transitions.
	c.After(log.day.AddDate(0, 0, 1).Sub(now))
	ui.Column(c).WidthPercent(100).MaxWidth(760).Margin(0, ui.Auto).Gap(16).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(5).Children(func() {
				ui.Text(c, log.day.Format("2006年1月2日")).FontSize(17).FontWeight(600)
				ui.Text(c, "今日完成按完成时间统计，明日计划按预计完成时间整理。").FontSize(12).TextColor(t.TextMuted)
			})
			button := webButton(c, "复制日志", true).Shrink(0)
			button.Children(func() { ui.Text(c, "复制日志") })
			if button.Clicked() {
				c.WriteClipboard(text)
				v.workLogCopied = text
			}
		})
		if v.workLogCopied == text {
			ui.Text(c, "已复制，可直接粘贴分享").FontSize(12).TextColor(t.Accent)
		}
		ui.Column(c).WidthPercent(100).Padding(22).Gap(12).Radius(t.Radius).Background(t.Alt).Border(1, t.Border).Children(func() {
			ui.Text(c, text).Label("工作日志预览").FontSize(14).LineHeight(1.8)
		})
	})
}
