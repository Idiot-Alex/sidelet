# macOS Dock 窗口列表 · build 51

用户反馈：右键 Dock 中的 Sidelet，窗口列表同时出现主窗口、侧栏助手和 MyGo 侧栏。

## 原因与修复

这些是同一应用的辅助窗口，不是多个 Sidelet 进程。当前 MyGo v0.5.1 的 macOS 后端将 `SetSkipTaskbar` 实现为空操作，不能依赖这个跨平台选项隐藏窗口菜单条目。

增加辅助窗口创建入口 `newAuxiliaryWindow`，在 macOS 创建后设置 `setExcludedFromWindowsMenu:YES`，并调用 `NSApplication.removeWindowsItem:` 清除已注册条目。主窗口继续直接使用 `mygo.NewWindow`。

覆盖精灵、任务侧栏、浮动卡片、助手菜单、快速添加及日期弹窗。保留窗口标题用于辅助功能和诊断，不改变焦点、透明度、置顶、Space 或拖动设置；非 macOS 平台的新增配置为空操作。

AppKit 的菜单排除属性见 [Apple 文档](https://developer.apple.com/documentation/appkit/nswindow/isexcludedfromwindowsmenu)。

## 验证与边界

- `go test ./cmd/sidelet ./internal/...`、`go vet` 以及 5 项 macOS 打包脚本测试通过。
- Mac 独立合成任务实例同时打开主窗口、侧栏、精灵与卡片：实际 Window 菜单仅包含一个 `Sidelet` 窗口条目。
- 从 File 菜单打开快速添加正常，标题输入框获得焦点；快速添加可见时 Window 菜单仍只有主窗口。
- Computer Use 两次读取 Dock 本身均超时，本轮直接验收的是 AppKit 的 Window 菜单，未把它表述为已完成 Dock 右键菜单的实际点击验收。
- Windows x64/ARM64 交叉编译通过；未触发 CI。
- build 51 完成签名、DMG 完整性与只读挂载后的包元数据校验；备份本机 build 50 后替换 `/Applications/Sidelet.app` 并启动，任务数据保留。

证据位于 `build/results/dock-windows-20261031/`，包含窗口菜单辅助功能树、运行日志和安装记录。安装包为 `build/releases/Sidelet-0.2.0-build51-local-arm64.dmg`，仍为未公证的本地签名预览版。

构建基于 `9beaa6a` 加本轮修改，`sourceDirty=true`。本轮未提交或推送。
