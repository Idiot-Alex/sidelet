# 提醒状态与原生界面整理

2026-10-23，build 39。沿用 Go + MyGo、三主题和透明侧栏，在现有界面上做小范围调整。

## 审查与改动

- Windows 正常的“由系统管理”说明原先会长期占据任务页顶部，还显示无法查询权限的“检查通知权限”。现在常规说明留在设置页；任务页只在存在未完成、未发送的提醒或正在编辑的提醒草稿时展示需要处理的问题。
- 未授权提示直接引导打开系统设置；发送 / 回执失败显示“重试提醒”，重试成功后清除错误。用独立错误状态区分正常说明与异常，不依赖文案字符串判断。
- 设置页的通知标签区分允许、系统管理、未开启 / 未允许及失败，颜色与文字一起表达状态。标题和标签居中对齐，按钮统一高度和图文行框，删除重复说明。
- 快速添加显示 Ctrl / Shift / Space 按键组合；入口改成与其他设置一致的紧凑按钮。页脚只显示产品名。
- 已检查任务页、820px 紧凑窗口、设置、未授权 / 失败状态与浮动卡片。保留现有布局与卡片，不为本轮审查扩大视觉改版范围。

## 验证

`TestNotificationUIOnlyInterruptsPendingReminders` 覆盖三主题、完成任务不遗留告警、提醒草稿引导和紧凑窗口文案 / 操作不重叠；`TestNotificationRetryClearsErrorWithoutLosingTask` 验证真实调度逻辑的失败保留、按钮触发重试、成功清除错误且仅发送一次。另验证未授权状态保留权限刷新入口，系统设置改变后可重新读取并更新标签。

本地全量回归 / race / vet 通过。使用 MyGo 原生渲染器生成合成资料图片，优化前后分别位于 `build/results/ui-reminders-before/` 与 `build/results/ui-reminders-after/`。这些是布局证据，不冒充桌面窗口截图或真实系统通知显示。

macOS ARM64 build 39 已从干净源码 `43a6714` 构建并校验 ad-hoc 签名，产物为 `build/bin/Sidelet.app`。使用内存合成任务实际启动、捕获自身主窗口 / 卡片内容并正常退出，最终证据在 `build/results/ui-reminders-macos-final/`。没有替换 `/Applications` 中的日常应用或修改用户资料。

生成方法：

```sh
SIDELET_UI_VISUAL_DIR="$PWD/build/results/ui-review" \
SIDELET_CARD_VISUAL_DIR="$PWD/build/results/ui-review" \
bash scripts/go.sh test ./cmd/sidelet -count=1 \
  -run '^(TestNativeUIVisualArtifacts|TestCardVisualArtifacts)$'
```

Windows build 39 从干净源码 `3751d1c` 构建，[x64 / ARM64 原生 CI](https://github.com/Idiot-Alex/sidelet/actions/runs/39827518998) 全部通过，最终 ZIP 和安装器已下载至 `build/releases/` 并复核。相对上述 Mac 构建，仅追加 Windows 诊断 / 测试改动，Mac 正式 UI 源码一致。通知专项每架构通过 9 项发送、回执、重启去重与撤回请求检查；横幅、点击与声音不在已通过范围内，详情见 [通知适配记录](WINDOWS_NOTIFICATIONS.md)。
