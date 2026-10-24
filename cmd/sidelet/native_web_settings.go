package main

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo/ui"
)

func webCheckbox(c *ui.Context, value *bool, label string, size float32, icons ...string) ui.Element {
	t := c.Theme()
	b := ui.CheckboxBase(c, value).Label(label).Gap(8).FontSize(size).LineHeight(4.0 / 3).AlignItems(ui.Center).TextColor(t.TextMuted)
	b.Children(func() {
		box := ui.Box(c).Size(14, 14).Radius(4).Border(1, t.TextMuted)
		if *value {
			box.Background(t.Accent).Border(1, t.Accent).TextColor(t.AccentText).Children(func() { webIcon(c, "check", 10) })
		}
		for _, name := range icons {
			webIcon(c, name, 14)
		}
		ui.Text(c, label).Height(size * 4 / 3).FixedLineHeight(size * 4 / 3)
	})
	return b
}

func webSwitch(c *ui.Context, value *bool, label string) ui.Element {
	t := c.Theme()
	bg, thumb, border := t.SurfaceHover, t.TextMuted, t.Border
	for _, visual := range webThemes {
		if visual.Theme == t {
			thumb = visual.Subtle
			break
		}
	}
	left := float32(2)
	if *value {
		bg, thumb, left = t.Accent, t.AccentText, 15
		border = t.Accent
	}
	b := ui.SwitchBase(c, value).Label(label).Size(32, 19).Radius(20).Border(1, border).Background(bg)
	b.Children(func() { ui.Box(c).Absolute().Left(left).Top(2).Size(13, 13).Radius(6.5).Background(thumb) })
	return b
}

func (v *nativeTasksView) webSettings(c *ui.Context) {
	t := v.visual
	width, _ := c.Size()
	vertical, horizontal := float32(18), float32(24)
	if width <= 900 {
		vertical, horizontal = 20, 20
	}
	ui.Column(c).WidthPercent(100).MaxWidth(850).Margin(0, ui.Auto).Radius(t.Radius).Background(t.Surface).Children(func() {
		if v.failed && v.status != "" {
			ui.Text(c, v.status).FontSize(12).TextColor(t.Danger).Background(t.DangerSoft).Padding(12, 16).Radius(t.Radius)
		}
		v.webAppearance(c)
		section := func(title string, body func(), status ...notificationPresentation) {
			ui.Column(c).Padding(vertical, horizontal).BorderColor(t.Border).BorderWidth(1, 0, 0, 0).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(0, 0, 10).Children(func() {
					ui.Text(c, title).Font(t.HeadingFont).FontSize(16).FontWeight(600)
					if len(status) > 0 {
						state := status[0]
						ui.Text(c, state.label).FontSize(11).FixedLineHeight(14).Padding(4, 7).Radius(5).TextColor(state.foreground).Background(state.background).Role(ui.RoleStatus)
					}
				})
				body()
			})
		}
		hint := func(text string) { ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).LineHeight(1.6).Margin(5, 0) }
		section("启动行为", func() {
			l := v.service.login
			status := "unsupported"
			if l != nil {
				status = l.status
			}
			ui.Row(c).Gap(28).AlignItems(ui.Center).Padding(15, 0).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "登录后启动 Sidelet").FontSize(13).FontWeight(500)
					hint("自动在后台运行，让桌面任务和提醒保持可用。")
					ui.Text(c, "系统状态："+loginLabel(status)).FontSize(12).TextColor(t.TextMuted)
				})
				on := loginRegistered(status)
				if webSwitch(c, &on, "登录后启动 Sidelet").Disabled(l == nil || status == "unsupported" || status == "unknown" || status == "otherLocation").Changed() {
					v.result(v.service.ChangeLogin(on), "")
				}
			})
			if status == "requiresApproval" {
				hint("请在系统登录项中允许 Sidelet；完成后返回这里检查状态。")
			}
			if status == "disabledBySystem" {
				hint("你已在系统中禁用 Sidelet 的自动启动；开启此开关将重新允许。")
			}
			if status == "otherLocation" {
				hint("请先在原位置的 Sidelet 中关闭登录启动，再在此处开启。")
			}
			if l != nil && l.error != "" && (!v.failed || v.status != l.error) {
				ui.Text(c, l.error).FontSize(12).TextColor(t.Danger).LineHeight(1.6).Margin(5, 0)
			}
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				for _, name := range []string{"打开系统登录项", "检查登录状态"} {
					b := webButton(c, name, false).Padding(8, 12).Background(t.Field).TextColor(t.Text).Disabled(l == nil || status == "unsupported")
					b.Children(func() { ui.Text(c, name) })
					if b.Clicked() {
						if name == "检查登录状态" {
							v.result(v.service.CheckLogin(), "")
						} else {
							v.result(v.service.OpenLoginSettings(), "")
						}
					}
				}
			})
			ui.Row(c).Gap(28).AlignItems(ui.Center).Padding(15, 0, 0).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, "启动时显示主窗口").FontSize(13).FontWeight(500)
					if runtime.GOOS == "windows" {
						hint("关闭后保留托盘入口与已固定的桌面任务。")
					} else {
						hint("关闭后保留菜单栏入口与已固定的桌面任务。")
					}
				})
				on := v.service.m.Preferences.Startup.ShowMainWindow
				if webSwitch(c, &on, "启动时显示主窗口").Changed() {
					value := v.service.m.Preferences
					value.Startup.ShowMainWindow = on
					_, err := v.service.SavePreferences(value)
					v.result(err, "")
				}
			})
		})
		section("新任务组默认值", func() {
			hint("仅在创建新任务组时使用。现有任务组的位置与密度可在“我的任务”中调整。")
			prefs := v.service.m.Preferences
			side := "右侧"
			if prefs.Edge.DefaultSide == "left" {
				side = "左侧"
			}
			density := "标准"
			if prefs.Edge.DefaultDensity == "compact" {
				density = "紧凑"
			}
			if prefs.Edge.DefaultDensity == "relaxed" {
				density = "宽松"
			}
			for i, item := range []struct {
				label   string
				value   *string
				options []string
			}{
				{"默认屏幕边缘", &side, []string{"左侧", "右侧"}}, {"默认标签密度", &density, []string{"紧凑", "标准", "宽松"}},
			} {
				row := ui.Row(c).AlignItems(ui.Center).Padding(15, 0)
				if i == 1 {
					row.Padding(15, 0, 0).Margin(8, 0, 0).BorderColor(t.Border).BorderWidth(1, 0, 0, 0)
				}
				row.Children(func() {
					ui.Text(c, item.label).FontSize(13).FontWeight(500).Grow(1)
					field := webSelect(c, item.value, item.options).Label(item.label).MinWidth(128).Height(36).Background(t.Field)
					if field.Changed() {
						value := v.service.m.Preferences
						if item.label == "默认屏幕边缘" {
							value.Edge.DefaultSide = "right"
							if side == "左侧" {
								value.Edge.DefaultSide = "left"
							}
						} else {
							value.Edge.DefaultDensity = "normal"
							if density == "紧凑" {
								value.Edge.DefaultDensity = "compact"
							}
							if density == "宽松" {
								value.Edge.DefaultDensity = "relaxed"
							}
						}
						_, err := v.service.SavePreferences(value)
						v.result(err, "")
					}
				})
			}
		})
		notification := v.notificationPresentation()
		section("系统通知", func() {
			hint("仅勾选“提醒我”的任务会发送通知，Sidelet 需在后台运行。")
			if notification.message != "" {
				line := ui.Text(c, notification.message).FontSize(12).LineHeight(1.6).Margin(5, 0).TextColor(t.TextMuted)
				if notification.attention {
					line.TextColor(notification.foreground)
				}
			}
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				if action := notification.action; action != "" && action != "打开系统通知设置" {
					b := webButton(c, action, false).Height(36).Padding(8, 12).Background(t.Field).Disabled(v.service.notices == nil)
					b.Children(func() { webActionText(c, action) })
					if b.Clicked() {
						v.notificationAction(action)
					}
				}
				b := webButton(c, "打开系统通知设置", false).Height(36).Padding(8, 12).Gap(6).Background(t.Field)
				b.Children(func() { webIcon(c, "settings", 14); webActionText(c, "打开系统通知设置") })
				if b.Clicked() {
					v.notificationAction("打开系统通知设置")
				}
				if v.service.notificationAuthorization == "denied" && !v.service.notificationFailed {
					b = webButton(c, "检查通知权限", false).Height(36).Padding(8, 12).Background(t.Field).Disabled(v.service.notices == nil)
					b.Children(func() { webActionText(c, "检查通知权限") })
					if b.Clicked() {
						v.notificationAction("检查通知权限")
					}
				}
			})
			if v.service.notificationAuthorization == "authorized" {
				hint("在系统通知列表中选择 Sidelet，调整横幅和声音。")
			}
		}, notification)
		section("快速添加", func() {
			if v.service.quickShortcut != nil && v.service.quickShortcut.diagnostic {
				hint("诊断模式使用 Control + Option + Shift + F19，仅用于独立注册检查。")
			}
			if v.service.quickShortcut != nil && v.service.quickShortcut.error != "" {
				ui.Text(c, v.service.quickShortcut.error).FontSize(12).TextColor(t.Danger).LineHeight(1.6).Margin(5, 0)
				retry := webButton(c, "重试快捷键", false).Padding(8, 12).Background(t.Field).Disabled(v.service.quickShortcut.retry == nil)
				retry.Children(func() { ui.Text(c, "重试快捷键") })
				if retry.Clicked() && v.service.quickShortcut.retry != nil {
					v.service.quickShortcut.retry()
				}
			} else if v.service.quickShortcut != nil && v.service.quickShortcut.registered {
				hint("快捷键已启用。")
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(4, 0, 8).Children(func() {
				for i, key := range []string{"Ctrl", "Shift", "Space"} {
					if i > 0 {
						ui.Text(c, "+").FontSize(11).TextColor(t.Subtle)
					}
					ui.Text(c, key).FontSize(12).FixedLineHeight(16).Padding(5, 8).Border(1, t.Border).Radius(5).Background(t.Field)
				}
				ui.Text(c, "随时记下任务").FontSize(12).TextColor(t.TextMuted).Margin(0, 0, 0, 8)
			})
			hint("Enter 保存，Esc 返回之前的应用。支持“明天下午3点 联系客户”等简单时间。")
			b := webButton(c, "打开快速添加", false).AlignSelf(ui.Start).Height(36).Padding(8, 12).Gap(6).Margin(12, 0, 0).Background(t.Field).TextColor(t.Text)
			b.Children(func() { webIcon(c, "plus", 14); webActionText(c, "打开快速添加") })
			if b.Clicked() && v.onQuickAdd != nil {
				v.onQuickAdd()
			}
		})
		section("导出任务", func() {
			hint("导出全部已保存任务。JSON 保留完整内容与桌面布局，CSV 适合用表格查看。")
			ui.Row(c).Gap(8).Margin(12, 0, 0).Children(func() {
				for _, name := range []string{"导出 JSON", "导出 CSV"} {
					b := webButton(c, name, false).Padding(8, 12).Gap(7).Background(t.Field).TextColor(t.Text).Disabled(v.exportBusy() || v.onExport == nil)
					b.Children(func() { webIcon(c, "download", 16); ui.Text(c, name) })
					if b.Clicked() {
						format := "json"
						if name == "导出 CSV" {
							format = "csv"
						}
						v.onExport(format)
					}
				}
			})
			hint("未提交的草稿不包含在导出文件中。")
			if e := v.service.export; e != nil && e.status != "" {
				color := t.TextMuted
				if e.failed {
					color = t.Danger
				}
				ui.Text(c, e.status).FontSize(12).TextColor(color).LineHeight(1.6).Margin(8, 0, 0)
			}
		})
		ui.Text(c, "Sidelet").FontSize(11).TextColor(t.Subtle).AlignSelf(ui.Center).Padding(10, 0, 18)
	})
}

func (v *nativeTasksView) webPosition(c *ui.Context) {
	t := v.visual
	ui.Column(c).WidthPercent(100).Margin(4, 0, 0).Padding(0, 16).Disabled(v.service.m.Arranging).Children(func() {
		b := webButton(c, "桌面位置", false).AlignSelf(ui.Start).Height(40).Border(0, ui.Transparent).Padding(8, 0).Gap(7)
		b.Children(func() {
			webIcon(c, "layout", 16)
			ui.Text(c, "桌面位置")
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
			ui.Row(c).WidthPercent(100).Gap(16).AlignItems(ui.End).Padding(4, 0, 16).Children(func() {
				m := v.service.m
				side := "左侧"
				if m.Side == "right" {
					side = "右侧"
				}
				ui.Column(c).Grow(1).Gap(8).Children(func() {
					ui.Text(c, "屏幕边缘").FontSize(12).TextColor(t.TextMuted)
					edge := webSelect(c, &side, []string{"左侧", "右侧"}).Label("桌面边缘").Height(36).Background(t.Field)
					if edge.Changed() {
						value := "left"
						if side == "右侧" {
							value = "right"
						}
						_, err := v.service.Layout(value, m.Offset, m.itemHeight())
						v.result(err, "")
					}
				})
				ui.Column(c).Grow(1).Gap(8).Children(func() {
					ui.Text(c, fmt.Sprintf("中心位置 %d%%", int(m.Offset*100+.5))).FontSize(12).TextColor(t.TextMuted)
					offset := m.Offset
					slider := ui.Slider(c, &offset, 0, 1).Label("桌面中心位置").Height(36)
					if slider.Changed() {
						_, err := v.service.Layout(m.Side, offset, m.itemHeight())
						v.result(err, "")
					}
				})
				ui.Column(c).Grow(1).Gap(8).Children(func() {
					ui.Text(c, "标签密度").FontSize(12).TextColor(t.TextMuted)
					density := "标准"
					if m.itemHeight() == 40 {
						density = "紧凑"
					}
					if m.itemHeight() == 56 {
						density = "宽松"
					}
					field := webSelect(c, &density, []string{"紧凑", "标准", "宽松"}).Label("桌面标签密度").Height(36).Background(t.Field)
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
			})
		}
	})
}
