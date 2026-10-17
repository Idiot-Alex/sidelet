# Windows 原生通知适配

更新：2026-10-17。主线为 Go + MyGo v0.5.1，无 WebView。

## 实现与行为

Windows 不再使用 `unsupportedNotifications`，而是调用 MyGo 的 `NewNotification().Show()` / `Close()`。当前 SDK 使用 `Shell_NotifyIconW` 通知区域接口，不是 WinRT Toast API；不增加 PowerShell、WebView、COM 注册或快捷方式安装步骤。

提醒继续由 `internal/reminder.Sync` 调度。系统接口返回成功后，原有 SQLite 事务保存提醒回执；返回错误时不消耗提醒，沿用一分钟后重试。已保存回执的任务在重启后不会重新发送。现有通知点击事件继续按资料标识打开主窗口。

Windows 设置页显示“由系统管理”和“刷新提醒状态”，不会把接口可用显示成“已允许”。“打开系统通知设置”使用 `ms-settings:notifications`。此接口没有 macOS 风格的应用授权请求；本实现不修改系统通知设置。

标题最多 63 个 UTF-16 单元，正文最多 255 个单元，预留原生字符串的结尾 NUL。截断保留完整 Unicode 字符，移除嵌入 NUL，避免长标题截断 emoji 的代理对。

## 回执与清理边界

- `State.Accepted` 仅记录本次进程中成功提交的标识，不代表通知已显示，也不冒充通知中心历史；`SystemDelivered` 不用这份记录计算可见通知数量。
- 发送成功、写入 SQLite 失败时，本进程内重试只补写回执，不再次提交。进程恰在两者之间崩溃时，因为无法读取系统历史，重启后仍可能重复一次。
- 完成、删除或推迟任务时，只撤销本次进程记录中的旧提醒，按资料前缀隔离。未知标识不会调用原生撤销接口；不调用全局 `ClearNotifications()`。
- 系统接口成功只表示提交成功。Windows 若因通知开关、勿扰模式或其他规则抑制显示，应用可能无法得知，不会自动补发。当前不能读取每个应用的允许/拒绝状态。
- 通知区域接口不能枚举或清理上次进程遗留的完整通知中心历史，也没有应用退出后的冷启动激活支持。
- 应用需保持运行；关闭主窗口仍可提醒，退出应用后不再发送。本轮未增加 Windows 睡眠唤醒 / 系统时间变化观察器。

这些限制属于当前 SDK 通知实现的能力边界，不能据此宣称 Windows 发布验收已完成。若后续需要完整历史、权限检测和退出后点击激活，应另做 WinRT Toast / 应用身份与安装器方案。

## 验证

在 macOS 上用可替换的原生提交函数验证通知核心与真实调度器的组合：失败保留待提醒状态、成功后回执失败的恢复、已有回执的重启去重、跨资料清理隔离、重复提交防护、中文 / emoji 的原生长度边界。设置页测试验证 Windows 状态不显示“已允许”或申请权限按钮。测试使用临时资料，不操作日常任务，不发送真实系统通知。

```sh
bash scripts/test-regression.sh
bash scripts/build-windows.sh amd64
bash scripts/build-windows.sh arm64
```

2026-10-17 本轮结果：全量回归 / race、Go vet、MyGo UI vet 全部通过；amd64 / arm64 无 cgo 构建与目标平台 vet 均通过，产物分别为 `build/bin/Sidelet-amd64.exe` 和 `build/bin/Sidelet-arm64.exe`。

本机不能运行 Windows EXE。交叉编译与测试不能验证 Windows 的弹出、声音、点击、撤销、通知开关和勿扰模式；这些仍待 Windows 实机验收。运行时检查需用 `-data-dir <独立目录> -main` 创建合成任务，并在系统通知开启 / 关闭、勿扰模式、完成 / 推迟、关闭主窗口、重启等场景逐项记录结果。

## 依据

- 当前依赖源码：`github.com/egoist/mygo@v0.5.1/internal/windows/tray.go`。
- Microsoft：<https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shell_notifyiconw>。
- Microsoft：<https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-notifyicondataw>。
- Microsoft 设置 URI：<https://learn.microsoft.com/en-us/windows/apps/develop/launch/launch-settings>。
