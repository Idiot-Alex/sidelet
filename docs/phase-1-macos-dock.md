# macOS Dock 图标与窗口恢复

2026-10-05，Sidelet 0.1.0 构建 5，macOS Apple Silicon。

## 行为

默认显示 Dock 图标。“设置 → 外观 → 在 Dock 中显示”即时生效并自动保存，退出 / 重启后保留。旧设置没有该字段时默认开启；主题、任务、登录启动和启动显示主窗口的原有选择不变。

关闭任务窗口仅隐藏它，桌面任务与后台提醒继续运行。点击 Dock 恢复关闭或最小化的主窗口，保留当前页面及未提交草稿；安静模式继续保持，隐藏的桌面标签和快速卡片不会被一起打开。关闭 Dock 图标后仍可通过菜单栏进入。

## 实现

偏好位于 `appearance.showDockIcon`，默认 `true`。沿用设置版本 1 和 SQLite 版本 2。保存前应用原生策略；文件保存失败会回退原策略，原生策略失败不会写入偏好；回退失败单独报告。

应用包继续用 `LSUIElement=true`、Accessory 策略启动，防止隐藏偏好在启动瞬间闪出 Dock 图标。窗口 ready 后根据已读取的偏好切换到 Regular（显示）或 Accessory（隐藏）。后台启动不主动激活应用或显示主窗口。原生策略仅在值变化时调用；Regular → Accessory 时恢复此前可见的设置窗口及其焦点，避免 AppKit 连带隐藏整个应用。

通过 Wails 的 ApplicationShouldHandleReopen 事件 hook 取消框架默认的“显示所有隐藏窗口”处理，再交给控制器恢复主窗口。该事件不重置页面；系统切换 Dock 策略可能产生同一事件，设置页仍应保留。菜单栏“我的任务”继续明确切回任务页。

`-trace-focus` 诊断增加 `activationPolicy`（0 为 Regular，1 为 Accessory）和主窗口可见性。正常启动不输出这些诊断。Windows 保留现有任务栏行为，不显示 macOS Dock 开关。

## 验证

证据目录：`build/results/dock-native-20261005/`。

- `regression.log`：前端检查 0 错误 / 0 警告，19 项前端测试，Go 竞态检查及 vet，29 项原生命中 / 焦点断言、16 项三主题透明背景断言全部通过。
- 设置测试新增旧设置兼容、显式开关重启恢复、原生失败不落盘、文件失败恢复 Dock、补偿失败、无关主题修改不触碰 Dock、非法 / 损坏设置不更改原生策略。设置测试共 6 项。
- 新增 `test-macos-dock.m/.sh` 并接入回归。9 项 AppKit 断言通过，覆盖 Dock 显隐、后台切换不显示窗口或抢焦点、隐藏 Dock 保留已打开设置窗口及键盘焦点、隐藏卡片不会被意外显示。夹具仅操作自己的窗口。
- `hidden-restart.log`：保存隐藏后不带 `-main` 重启，原生 activationPolicy=1，主窗口隐藏、应用未激活；UI 确认开关仍关闭。
- `visible-restart.log`：保存显示后后台重启，activationPolicy=0，主窗口仍隐藏、应用未激活。显示 / 隐藏即时切换的原生状态与偏好一致，最终版本隐藏时主窗口保持可见和焦点。
- 使用真实应用 UI 创建未提交草稿，关闭主窗口后通过系统重新打开事件恢复，草稿保留；最小化后也正常恢复。安静模式关闭主窗口再恢复后仍保持开启。
- `quiet-reopened-windows.json`：系统重新打开安静模式实例后，只读 WindowServer 诊断确认 Sidelet 仅有一扇层级 0 的主窗口，无浮动标签或快速卡片。
- Dock 系统 UI 的自动读取超时，未将鼠标点击 Dock 图标记录为自动化通过；这里用 `open /Applications/Sidelet.app` 触发同一原生 applicationShouldHandleReopen 路径，结合 AppKit 原生策略检查验证窗口恢复。
- `package.log`：最终应用签名和 DMG 校验通过。更新前备份应用为 `Sidelet-build4.app`，测试任务保留在独立资料目录。结束时正常资料目录的全部 SQLite 表及原有设置逐项核对一致，已恢复日常实例，默认显示 Dock。

安装应用与构建目录二进制一致，SHA-256：`9aa51c12e2ec95c08cbba8214cdc449b33ce73b2a9e47a061bd440806850a2dd`。

DMG：`build/releases/Sidelet-0.1.0-local-arm64.dmg`，SHA-256：`794690bfba96a63641f498d8e374a98c6dd223dcf9d73d0e2cd1ff1b1da78743`。

原有 P0 物理鼠标、多屏、Spaces 等验收状态保持不变。
