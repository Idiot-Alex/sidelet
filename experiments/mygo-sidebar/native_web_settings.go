package main

import (
	"fmt"
	"github.com/egoist/mygo/ui"
)

func webCheckbox(c *ui.Context, value *bool, label string, size float32, icons ...string) *ui.Element {
	t := c.Theme()
	b := ui.CheckboxBase(c, value).Label(label).Gap(8).FontSize(size).TextColor(t.TextMuted)
	b.Children(func() {
		box := ui.Box(c).Size(14, 14).Radius(4).Border(1, t.TextMuted)
		if *value {
			box.Background(t.Accent).Border(1, t.Accent).TextColor(t.AccentText).Children(func() { webIcon(c, "check", 10) })
		}
		for _, name := range icons {
			webIcon(c, name, 14)
		}
		ui.Text(c, label)
	})
	return b
}

func webSwitch(c *ui.Context, value *bool, label string) *ui.Element {
	t := c.Theme()
	bg, thumb := t.SurfaceHover, t.TextMuted
	left := float32(2)
	if *value {
		bg, thumb, left = t.Accent, t.AccentText, 15
	}
	b := ui.SwitchBase(c, value).Label(label).Size(32, 19).Radius(20).Border(1, t.Border).Background(bg)
	b.Children(func() { ui.Box(c).Absolute().Left(left).Top(2).Size(13, 13).Radius(6.5).Background(thumb) })
	return b
}

func (v *nativeTasksView) webSettings(c *ui.Context) {
	t := v.visual
	ui.Column(c).WidthPercent(100).MaxWidth(850).Margin(0, ui.Auto).Gap(16).Children(func() {
		v.webAppearance(c)
		section := func(title string, body func()) {
			ui.Column(c).Padding(22, 24).Border(1, t.Border).Radius(t.Radius).Background(t.Surface).Children(func() { ui.Text(c, title).Font(t.HeadingFont).FontSize(16).FontWeight(600).Margin(0, 0, 14); body() })
		}
		hint := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).LineHeight(1.6).Margin(5, 0) }
		row := func(title, description string, on, disabled bool) {
			ui.Row(c).Gap(28).Padding(15, 0).Children(func() {
				ui.Column(c).Grow(1).Children(func() { ui.Text(c, title).FontSize(13).FontWeight(500); hint(description) })
				webSwitch(c, &on, title).Disabled(disabled)
			})
		}
		buttons := func(names ...string) {
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				for _, name := range names {
					b := webButton(c, name, false).Padding(8, 12).Background(t.Field).TextColor(t.Text).Disabled(true)
					b.Children(func() { ui.Text(c, name) })
				}
			})
		}
		section("启动行为", func() {
			row("登录 Mac 后启动 Sidelet", "自动在后台运行，让桌面任务和提醒保持可用。", false, true)
			ui.Text(c, "系统状态：原型尚未接入").FontSize(11).TextColor(t.TextMuted)
			hint("测试资料目录不更改系统登录项，请在正常版本中设置。")
			buttons("打开系统登录项", "检查登录状态")
			ui.Box(c).Height(1).Background(t.Border).Margin(15, 0, 8)
			row("启动时显示主窗口", "关闭后保留菜单栏入口与已固定的桌面任务。", true, true)
		})
		section("新任务组默认值", func() {
			hint("仅在创建新任务组时使用。现有任务组的位置与密度可在“我的任务”中调整。")
			for _, item := range []struct {
				label, value string
				options      []string
			}{{"默认屏幕边缘", "右侧", []string{"左侧", "右侧"}}, {"默认标签密度", "标准", []string{"紧凑", "标准", "宽松"}}} {
				ui.Row(c).Padding(15, 0).Children(func() {
					ui.Text(c, item.label).FontSize(13).FontWeight(500).Grow(1)
					value := item.value
					ui.Select(c, &value, item.options).Label(item.label).MinWidth(128).Height(36).Background(t.Field).Disabled(true)
				})
			}
		})
		section("系统通知", func() {
			hint("仅勾选“提醒我”的任务会发送通知，Sidelet 需在后台运行。")
			if v.service.notificationStatus != "" {
				hint(v.service.notificationStatus)
			} else if v.service.notificationAuthorization == "authorized" {
				hint("系统状态：已允许")
			}
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				b := webButton(c, "检查通知权限", false).Padding(8, 12).Background(t.Field)
				b.Children(func() { ui.Text(c, "检查通知权限") })
				if b.Clicked() && v.service.notices != nil {
					v.service.notices.request()
				}
				if v.service.notificationAuthorization == "notDetermined" {
					b = webButton(c, "开启系统通知", false).Padding(8, 12).Background(t.Field)
					b.Children(func() { ui.Text(c, "开启系统通知") })
					if b.Clicked() && v.service.notices != nil {
						v.service.notices.permission()
					}
				}
				b = webButton(c, "打开系统通知设置", false).Padding(8, 12).Background(t.Field)
				b.Children(func() { ui.Text(c, "打开系统通知设置") })
				if b.Clicked() {
					openNotificationSettings()
				}
			})
			hint("在系统通知列表中选择 Sidelet MyGo Lab，调整横幅和声音。")
		})
		section("快速添加", func() {
			hint("在任何应用中按 Control + Shift + Space，记下任务。Enter 保存，Esc 取消并返回之前的应用。")
			hint("支持“明天下午3点 联系客户”等简单时间，默认不固定到桌面、不发送系统通知。")
			b := webButton(c, "打开快速添加", false).Padding(8, 12).Margin(12, 0, 0).Background(t.Field).TextColor(t.Text)
			b.Children(func() { ui.Text(c, "打开快速添加") })
			if b.Clicked() && v.onQuickAdd != nil {
				v.onQuickAdd()
			}
		})
		section("导出任务", func() {
			hint("导出全部任务，包含已完成、未固定和暂时隐藏的任务。JSON 保留完整任务字段与桌面布局，CSV 适合用表格查看。")
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				for _, name := range []string{"导出 JSON", "导出 CSV"} {
					b := webButton(c, name, false).Padding(8, 12).Gap(7).Background(t.Field).TextColor(t.Text).Disabled(true)
					b.Children(func() { webIcon(c, "download", 16); ui.Text(c, name) })
				}
			})
			hint("导出的是点击时已保存的数据，不包含未提交的草稿。当前版本暂不支持导入。")
		})
		ui.Text(c, "Sidelet MyGo · UI 对齐原型 · 灰色控件尚未接入系统能力").FontSize(11).TextColor(t.Subtle).AlignSelf(ui.Center).Padding(6, 0, 0)
	})
}

func (v *nativeTasksView) webPosition(c *ui.Context) {
	t := v.visual
	ui.Column(c).Margin(14, 0, 0).Padding(0, 16).Border(1, t.Border).Radius(t.Radius).Disabled(v.service.m.Arranging).Children(func() {
		b := webButton(c, "桌面位置", false).Border(0, ui.Transparent).Padding(15, 0).Gap(7)
		b.Children(func() {
			webIcon(c, "layout", 16)
			ui.Text(c, "桌面位置").Grow(1)
			side := "左侧"
			if v.service.m.Side == "right" {
				side = "右侧"
			}
			ui.Text(c, side).FontSize(11)
			webIcon(c, "chevron", 12)
		})
		if b.Clicked() {
			v.positionOpen = !v.positionOpen
		}
		if v.positionOpen {
			ui.Column(c).Gap(10).Padding(0, 0, 16).Children(func() {
				ui.Text(c, "屏幕边缘").FontSize(12).TextColor(t.TextMuted)
				m := v.service.m
				side := "左侧"
				if m.Side == "right" {
					side = "右侧"
				}
				edge := ui.Select(c, &side, []string{"左侧", "右侧"}).Label("桌面边缘").Height(36).Background(t.Field)
				if edge.Changed() {
					value := "left"
					if side == "右侧" {
						value = "right"
					}
					_, err := v.service.Layout(value, m.Offset, m.itemHeight())
					v.result(err, "")
				}
				ui.Text(c, fmt.Sprintf("中心位置 %d%%", int(m.Offset*100+.5))).FontSize(12).TextColor(t.TextMuted)
				offset := m.Offset
				slider := ui.Slider(c, &offset, 0, 1).Label("桌面中心位置")
				if slider.Changed() {
					_, err := v.service.Layout(m.Side, offset, m.itemHeight())
					v.result(err, "")
				}
				ui.Text(c, "标签密度").FontSize(12).TextColor(t.TextMuted)
				density := "标准"
				if m.itemHeight() == 40 {
					density = "紧凑"
				}
				if m.itemHeight() == 56 {
					density = "宽松"
				}
				field := ui.Select(c, &density, []string{"紧凑", "标准", "宽松"}).Label("桌面标签密度").Height(36).Background(t.Field)
				if field.Changed() {
					height := 44
					if density == "紧凑" {
						height = 40
					}
					if density == "宽松" {
						height = 56
					}
					_, err := v.service.Layout(m.Side, m.Offset, height)
					v.result(err, "")
				}

			})
		}
	})
}
