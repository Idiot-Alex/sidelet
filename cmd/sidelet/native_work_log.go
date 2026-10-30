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
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	t := v.visual
	c.After(today.AddDate(0, 0, 1).Sub(now))
	if v.workLogFrom == "" {
		v.workLogFrom = today.AddDate(0, 0, -6).Format("2006-01-02")
		v.workLogTo = today.Format("2006-01-02")
	}
	copyText := func(text string) { c.WriteClipboard(text); v.workLogCopied = text; c.Announce("日志已复制") }
	ui.Column(c).WidthPercent(100).MaxWidth(760).Margin(0, ui.Auto).Gap(18).Children(func() {
		ui.Column(c).Gap(5).Children(func() {
			ui.Text(c, "完成记录").FontSize(21).FontWeight(600)
			ui.Text(c, "按实际完成日期归档，保留完成时的任务名称。").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			for _, days := range []int{7, 30} {
				b := webButton(c, fmt.Sprintf("最近 %d 天", days), false)
				b.Children(func() { ui.Text(c, fmt.Sprintf("最近 %d 天", days)) })
				if b.Clicked() {
					v.workLogFrom = today.AddDate(0, 0, 1-days).Format("2006-01-02")
					v.workLogTo = today.Format("2006-01-02")
					v.workLogCopied = ""
				}
			}
			previousFrom, previousTo := v.workLogFrom, v.workLogTo
			webTextInput(c.Key("log-from"), &v.workLogFrom).Label("日志开始日期").Placeholder("YYYY-MM-DD").Tooltip("开始日期，格式 YYYY-MM-DD").Width(124).Height(34)
			ui.Text(c, "至").TextColor(t.Subtle)
			webTextInput(c.Key("log-to"), &v.workLogTo).Label("日志结束日期").Placeholder("YYYY-MM-DD").Tooltip("结束日期，包含当天").Width(124).Height(34)
			if previousFrom != v.workLogFrom || previousTo != v.workLogTo {
				v.workLogCopied = ""
			}
		})
		from, e1 := time.ParseInLocation("2006-01-02", v.workLogFrom, now.Location())
		to, e2 := time.ParseInLocation("2006-01-02", v.workLogTo, now.Location())
		if e1 != nil || e2 != nil || from.After(to) || to.After(today) {
			ui.Text(c, "请输入有效日期（YYYY-MM-DD），开始日期不能晚于结束日期，结束日期不能晚于今天。").FontSize(12).TextColor(t.Danger)
			return
		}
		days := workLogTimeline(v.service.m.workCompletions(), from, to)
		text := workLogRangeText(days, from, to)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			count := 0
			for _, day := range days {
				count += len(day.entries)
			}
			ui.Text(c, fmt.Sprintf("%s — %s · %d 项完成", from.Format("1月2日"), to.Format("1月2日"), count)).FontSize(12).TextColor(t.TextMuted).Grow(1)
			b := webButton(c, "复制日志", true)
			b.Children(func() { ui.Text(c, "复制所选日期") })
			if b.Clicked() {
				copyText(text)
			}
		})
		if v.workLogCopied != "" {
			ui.Text(c, "已复制，可直接粘贴分享").Role(ui.RoleStatus).FontSize(12).TextColor(t.Accent)
		}
		// Tomorrow is live planning, not a historical snapshot.
		plan := generateWorkLog(v.service.m.Tasks, now)
		ui.Column(c).Padding(16).Gap(9).Radius(t.Radius).Background(t.Soft).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "明日计划 · "+today.AddDate(0, 0, 1).Format("1月2日")).FontSize(14).FontWeight(550).Grow(1)
				b := webButton(c, "复制明日计划", false).Border(0, ui.Transparent).Padding(4, 8)
				b.Children(func() { ui.Text(c, "复制计划") })
				if b.Clicked() {
					var b strings.Builder
					fmt.Fprintf(&b, "明日计划 · %s\n", today.AddDate(0, 0, 1).Format("2006年1月2日"))
					for _, item := range plan.planned {
						fmt.Fprintf(&b, "- %s %s\n", time.UnixMilli(item.DueAt).In(now.Location()).Format("15:04"), strings.Join(strings.Fields(item.Title), " "))
					}
					if len(plan.planned) == 0 {
						b.WriteString("暂无预计明天完成的任务")
					}
					copyText(strings.TrimSpace(b.String()))
				}
			})
			if len(plan.planned) == 0 {
				ui.Text(c, "暂无预计明天完成的任务").FontSize(12).TextColor(t.TextMuted)
			}
			for _, item := range plan.planned {
				ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
					ui.Text(c, time.UnixMilli(item.DueAt).In(now.Location()).Format("15:04")).FontSize(12).TextColor(t.Accent).Width(42).Shrink(0)
					ui.Text(c, item.Title).FontSize(13).Grow(1).MinWidth(0)
					if item.Priority > 0 {
						ui.Text(c, priorityName(item.Priority)).FontSize(11).TextColor(t.TextMuted).Shrink(0)
					}
				})
			}
		})
		ui.Column(c).Label("工作日志预览").Gap(0).Children(func() {
			if len(days) == 0 {
				ui.Text(c, "这段时间暂无完成记录").Padding(24, 0).FontSize(14).TextColor(t.TextMuted)
			}
			for _, day := range days {
				ui.Row(c.Key(day.day.Format("2006-01-02"))).Gap(14).AlignItems(ui.Stretch).Children(func() {
					ui.Column(c).Width(14).Shrink(0).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(10, 10).Margin(12, 0, 0).Radius(5).Background(t.Accent)
						ui.Box(c).Width(1).Grow(1).Background(t.Border)
					})
					ui.Column(c).Grow(1).MinWidth(0).Padding(0, 0, 24).Gap(10).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
							label := day.day.Format("2006年1月2日")
							if day.day.Equal(today) {
								label = "今天 · " + day.day.Format("1月2日")
							} else if day.day.Equal(today.AddDate(0, 0, -1)) {
								label = "昨天 · " + day.day.Format("1月2日")
							}
							ui.Text(c, label).FontSize(15).FontWeight(550).Grow(1)
							ui.Text(c, fmt.Sprintf("%d 项", len(day.entries))).FontSize(12).TextColor(t.Subtle)
							b := webButton(c, "复制日志："+day.day.Format("2006-01-02"), false).Border(0, ui.Transparent).Padding(6, 8)
							b.Children(func() { ui.Text(c, "复制当天") })
							if b.Clicked() {
								copyText(day.text())
							}
						})
						for _, entry := range day.entries {
							ui.Row(c).Gap(14).AlignItems(ui.Start).Children(func() {
								ui.Text(c, time.UnixMilli(entry.CompletedAt).In(now.Location()).Format("15:04")).Width(42).Shrink(0).FontSize(12).TextColor(t.Subtle)
								ui.Text(c, entry.Title).FontSize(14).LineHeight(1.6).Grow(1).MinWidth(0)
							})
						}
					})
				})
			}
		})
		earlier := false
		for _, entry := range v.service.m.workCompletions() {
			if entry.CompletedAt < from.UnixMilli() {
				earlier = true
				break
			}
		}
		if earlier {
			b := webButton(c, "加载更早记录", false).AlignSelf(ui.Center)
			b.Children(func() { ui.Text(c, "再往前查看 7 天") })
			if b.Clicked() {
				v.workLogFrom = from.AddDate(0, 0, -7).Format("2006-01-02")
				v.workLogCopied = ""
			}
		}
	})
}
