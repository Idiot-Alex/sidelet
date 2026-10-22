# Windows 跨进程系统输入验证

2026-10-23，build 38。补足此前同进程窗口与定向 `SendMessage` 检查未覆盖的跨进程输入队列、系统鼠标/键盘路由和光标恢复。

## 测试方式

`TestPackagedWindowsCrossProcessFocus` 启动 ZIP 中的实际 EXE 两次：一个运行完整原生 Sidelet 和独立 SQLite 合成资料，另一个使用现有 `-native-main=false -probe-only` 模式提供输入框。两个进程的 PID、UI 线程与 Windows 输入队列独立；伴随窗口仍使用 MyGo 控件，不冒充第三方应用兼容性验收。

操作使用 Windows `SendInput`：鼠标移动/点击、Ctrl+A、Enter、Escape，以及正式的 Ctrl+Shift+Space 全局快捷键。文字通过 Unicode 键盘事件输入；不直接改运行中 model、不发送控件按钮消息、不使用 `AttachThreadInput` 或放宽前台权限。测试读取应用自身的诊断日志与合成 SQLite 状态。

后台输入框先输入 `AB`，按左方向键将光标放在中间。每次侧栏或卡片操作之后继续输入一个字符，必须插入在原来的 `B` 前面；因此不只检查前台 HWND，也检查输入确实恢复到同一字段和光标位置。

覆盖：

1. 悬停侧栏后继续后台输入。
2. 点击任务预览后继续后台输入。
3. 鼠标进入只读卡片并停留超过关闭宽限后继续输入。
4. 编辑卡片、点击取消，恢复原输入与光标，丢弃草稿。
5. 再次编辑并按 Enter 保存，恢复原输入与光标。
6. 正式全局快捷键打开快速添加，输入后 Escape 取消，再继续后台输入。
7. 快捷键打开快速添加并 Enter 保存，再继续后台输入。
8. 点击后台输入框使快速添加失焦，尊重用户当前窗口选择。
9. 重新打开快速添加，保留失焦草稿；取消后继续原输入。
10. SQLite 最终只包含已确认保存的卡片编辑和新增任务。

## 隔离与限制

- 所有系统输入代码仅在 `_test.go` 中，不进入安装包。
- 默认跳过；既要显式提供 `SIDELET_SMOKE_EXE`，也必须为 GitHub 托管的临时 Windows 主机。不会在本地个人桌面或自托管机器注入输入。
- 每批键盘事件先核对精确前台 HWND 和已启动的进程；鼠标点击先核对实际指针位置与系统命中窗口，遮挡或失焦即失败，不尝试操作未知窗口。
- 退出释放测试按键、恢复指针，只终止自己启动的两个进程；合成任务和日志保留在随机 `cross-process-*` 目录。
- 结果不涵盖物理键鼠、中文输入法组合/候选、多屏、不同权限等级或第三方应用的特殊控件；也不是注销/登录测试。

## 验证结果

源码 `641534f` 的 [Windows 原生 CI](https://github.com/Idiot-Alex/sidelet/actions/runs/39819946040) 两架构全部通过：x64 全流程 3 分 15 秒，ARM64 4 分钟；上述 10 项跨进程检查分别用时 4.09 秒和 4.34 秒。原有 18 项窗口输入检查、4 种打包 EXE 启动呈现、持久化与安装器生命周期继续通过。本轮未发现需要修改正式应用焦点逻辑的问题。

build 38 的两架构 ZIP、安装器及 `.sha256` 已下载到 `build/releases/`，重新核对 SHA-256、ZIP CRC、四文件清单、版本 / 架构和干净源码提交。完整记录为 `build/results/windows-focus-39819946040/verified.json`，原始 `cross-process.json`、`cross-process-focus.jsonl` 及应用/伴随进程日志保存在同目录的两架构诊断文件夹。

| 安装器 | SHA-256 |
| --- | --- |
| `Sidelet-0.2.0-build38-windows-amd64-local-setup.exe` | `2a0fa54de58c64308618439b43d077edbe8c638e7ca2d98733b092880e73883c` |
| `Sidelet-0.2.0-build38-windows-arm64-local-setup.exe` | `1729bbc07c6061b04984b1d3b3a4e19d82518f49c089d6ef6bb63ed9ee1b9087` |

依据：[Microsoft SendInput](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendinput)、[SetForegroundWindow 的前台限制](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setforegroundwindow)。
