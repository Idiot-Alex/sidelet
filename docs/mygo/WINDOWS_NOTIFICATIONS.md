# Windows 原生通知适配

更新：2026-10-23，build 39。主线为 Go + MyGo v0.5.1，无 WebView。

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

### build 39：打包 EXE 的实际通知链路

`TestPackagedWindowsNotifications` 在 GitHub 托管临时 Windows 桌面运行 ZIP 中的实际 EXE，所有任务与 SQLite 都在随机独立目录。检查主窗口隐藏时的逾期提醒和定时到期提醒、回执落库、禁用 / 完成 / 未到期任务隔离、正常退出后真实回执去重，以及通过系统鼠标点击卡片完成 / 推迟任务后的撤回请求和持久化。默认跳过，不在个人桌面或自托管主机注入输入。

诊断字段 `notificationSession` 记录成功提交次数、失败次数、本进程接收标识与撤回请求数；只在既有诊断日志开启时输出，不包含系统通知历史，也不把撤回请求当成可见横幅消失的证明。

`probe-windows-notification.ps1` 仅在临时主机的 Shell 窗口中匹配随机合成通知标题；发现可见横幅时尝试调用其最近按钮的 UI Automation InvokePattern，再检查正式通知点击回调是否打开主窗口。不会伪造 Shell 回调、不打开通知中心、不修改通知开关。横幅显示与激活独立写入 `notifications.json`；托管环境未显示或不支持自动化时保留未验收状态。

本轮本地回归、race、vet、Windows 双架构编译通过。源码 `3751d1c` 的 [Windows 原生复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39827518998) 两架构全部通过：x64 3 分 24 秒、ARM64 5 分 27 秒；通知专项分别耗时 31.38 / 32.34 秒，每个架构通过以下 9 项：

1. 主窗口隐藏时提交逾期提醒。
2. SQLite 保存回执，未到期、禁用、已完成任务不被消费。
3. 后续同步不重复提交。
4. 正常退出后重启，真实提交回执阻止再次发送。
5. 后台定时到期后提交提醒。
6. 卡片推迟生成新的提醒标识，并请求撤回旧提醒。
7. 推迟时间与待提醒状态保存到 SQLite。
8. 重启后不发送已确认或尚未到期的提醒。
9. 卡片完成状态保存，并请求撤回本进程接收的提醒。

首次执行暴露的是测试断言错误：推迟应更新 `SnoozedUntil` / `ReminderAt`，不修改任务原始 `DueAt`。修正断言后复验，未因此改动正式推迟逻辑。

横幅验收仍未完成：x64 探测正常结束，但未找到匹配的可见横幅；ARM64 探测进程退出码为 1、没有输出，不能判断横幅是否显示。两者均没有验证通知点击激活或声音。通知开关、勿扰模式、普通用户权限及物理点击仍需实机验证。

两架构 ZIP / 安装器已下载至 `build/releases/`，重新核对 SHA-256、ZIP CRC、四文件清单及干净源码信息。完整结果在 `build/results/windows-notifications-39827518998/verified.json`；原始 `notifications.json`、探测日志与合成 SQLite 位于同目录各架构诊断目录。

同轮的任务页与设置页改进见 [提醒状态 UI 记录](REMINDER_UI_20261023.md)。

## 依据

- 当前依赖源码：`github.com/egoist/mygo@v0.5.1/internal/windows/tray.go`。
- Microsoft：<https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shell_notifyiconw>。
- Microsoft：<https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-notifyicondataw>。
- Microsoft 设置 URI：<https://learn.microsoft.com/en-us/windows/apps/develop/launch/launch-settings>。
