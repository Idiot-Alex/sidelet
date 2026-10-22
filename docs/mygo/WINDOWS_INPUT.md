# Windows 侧栏输入适配

更新：2026-10-23，build 36。实现位于 `cmd/sidelet/input_windows.go`。

## 行为

- 侧栏窗口的 Win32 区域由可见标签、展开行、整理工具条和撤销提示组成。空白和圆角外侧不参与命中；展开行合并为连续区域，不在把手和文字之间留接缝。
- 侧栏图标的悬浮说明按实际布局临时加入绘制区域，隐藏后撤销，避免窗口裁剪截掉文字或遗留透明拦截区。
- 侧栏及只读卡片设置 `WS_EX_NOACTIVATE`。MyGo v0.5.1 的原生绘制子窗口在鼠标按下时还会显式调用 `SetFocus`，因此使用当前 UI 线程的 CBT hook，仅拒绝属于这两个被动窗口的激活和焦点请求。没有全局键鼠监听，也不操作其他应用窗口的消息过程。
- 鼠标离开窗口时结合系统命中结果更新标签 / 卡片停留状态，沿用 500ms 跨窗宽限。拖动根据每个已送达原生事件的屏幕坐标移动，包括窗口移动后局部坐标相同的连续事件；丢失捕获取消拖动，正常松手保存位置。
- 编辑时开放卡片焦点，保存 / 取消恢复之前的窗口。用户已经切换窗口时不夺回焦点。卡片沿用已有稳定锚点计算。
- 按窗口 DPI 创建区域，尺寸 / DPI 变化重新同步。空闲时不轮询鼠标。退出移除窗口 subclass、UI 线程 hook 和区域资源；适配失败时让窗口穿透，主任务窗口仍可使用，诊断记录错误。

## 自动检查

`scripts/test-windows-ci.ps1` 对实际 ZIP 中的 EXE 运行 `TestPackagedWindowsOverlayInput`。仅在独立 Windows 测试桌面运行：会把指针移动到应用自己的合成测试窗口，并向这些 HWND 发送鼠标按钮消息；结束恢复指针位置。没有键盘注入、外部窗口消息或真实资料读写。

检查包含空白 / 圆角命中、后台悬停展开、悬浮说明区域显示 / 撤销、SDK 焦点请求拦截、点击打开、跨窗停留、编辑 / 取消 / 再次保存、焦点切换保护、相同局部坐标拖动、释放捕获、捕获丢失与后续拖动、安静模式及资源清理。失败会返回非零并保留 `native-check.json`。

本地 macOS 回归（race / vet）、Windows 双架构测试程序交叉编译和 Windows vet 通过。最终源码 `12865c6` 的 [Windows CI](https://github.com/Idiot-Alex/sidelet/actions/runs/39809334284) 两架构全部通过：每个架构完成 18 项原生输入检查（含提示框显示 / 撤销区域），以及启动 / SQLite 恢复、ZIP / 版本资源、完整安装器生命周期。测试结果不包含真实输入法或硬件鼠标验收。

ARM64 托管镜像可能有首次登录窗口覆盖测试桌面，见 [runner-images #14069](https://github.com/actions/runner-images/issues/14069)。CI 会先运行 `prepare-windows-ci-desktop.ps1`，仅允许 GitHub 托管的 Windows ARM64 临时主机：关闭首次登录进程及性能设置窗口、抑制重复登录提示，再运行显式启用的 `TestPrepareWindowsCIDesktop`，发送 Escape 收起可见的 Shell 临时界面并最小化其余覆盖窗口，保留桌面和任务栏。普通本地运行和自托管主机不会执行这些操作。辅助程序在 `_test.go` 中，不包含在安装包内，准备过程完整记录到 CI 日志。

build 38 另增两个独立进程的系统鼠标 / 键盘检查，双架构均通过后台输入保持、卡片与快速添加保存 / 取消后的光标恢复，以及失焦草稿保留。具体方式和证据见 [跨进程输入记录](WINDOWS_CROSS_PROCESS_INPUT.md)。

这些检查不代表物理鼠标、第三方应用特殊输入控件、中文输入法候选、多显示器 / 混合 DPI 或普通用户权限已通过。没有对这些项目标记验收完成。

适配器通过公开 `NativeHandle` 定位 MyGo v0.5.1 的 `MyGoSurface` 子窗口；升级 SDK 时须复验该窗口类及事件行为。未修改 MyGo 依赖源码。

## 最终产物

两架构的 ZIP、安装器及 `.sha256` 已下载到 `build/releases/`，复验 SHA-256、ZIP CRC、四文件清单、架构 / 版本和干净源码提交 `12865c6`。本地完整记录为 `build/results/windows-input-39809334284/verified.json`，原生窗口及安装器报告保存在同目录的 `diagnostics/`。

| 安装器 | SHA-256 |
| --- | --- |
| `Sidelet-0.2.0-build36-windows-amd64-local-setup.exe` | `ff92bcc9954cb7907eb0f1bb2ecaf50ca987ceb968b697299b3eba213cd23a98` |
| `Sidelet-0.2.0-build36-windows-arm64-local-setup.exe` | `bc1be0750ee45cf1a1e6b8a72bed3dcfa0a2ff884425f400b4dc6c2d522b38c5` |

官方 API 依据：[SetWindowRgn](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowrgn)、[SetFocus](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setfocus)、[CBTProc](https://learn.microsoft.com/en-us/windows/win32/winmsg/cbtproc)、[SetWindowsHookExW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowshookexw)、[SetWindowSubclass](https://learn.microsoft.com/en-us/windows/win32/api/commctrl/nf-commctrl-setwindowsubclass)。
