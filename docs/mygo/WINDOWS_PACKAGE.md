# Windows 发布包基础

更新：2026-10-23；源码版本 0.2.0，build 43。产物为未签名测试包；普通用户交互实机验收尚未完成，各轮原生 CI 结果见 [记录](WINDOWS_CI.md)。

## 构建与打包

macOS / Linux，Go 1.27.1 或以上：

```sh
bash scripts/build-windows.sh amd64
bash scripts/package-windows.sh amd64
bash scripts/package-windows.sh arm64
```

Windows PowerShell，已安装相应 Go 工具链：

```powershell
.\scripts\build-windows.ps1 -Arch amd64
.\scripts\package-windows.ps1 -Arch amd64
.\scripts\package-windows.ps1 -Arch arm64
```

两个平台的脚本调用同一个 Go 打包工具。PowerShell 入口及产物进程启停、SQLite 恢复已在原生 Windows x64 / ARM64 CI 上运行通过，见 [CI 记录](WINDOWS_CI.md)。首次构建需下载固定版本的构建工具依赖，不需要安装 Windows SDK、资源编译器、NSIS、Python 或 Node.js。

裸 EXE 写到 `build/bin/Sidelet-<arch>.exe`。ZIP 写到 `build/releases/Sidelet-0.2.0-build43-windows-<arch>-local.zip`，同目录有 `.zip.sha256`。

ZIP 中只有一个同名目录及以下四个文件，不包含任务资料：

- `Sidelet.exe`
- `使用说明.txt`：运行、独立资料、通知、退出和校验方法。
- `实机验收.md`：启动、图标、三主题、中文输入、快捷键、侧栏、提醒、数据保存、屏幕与缩放等检查表，初始全部标为待测。
- `build-info.json`：版本、构建号、架构、Go 版本、Go 提供的 Git 源码信息、未签名标记及待实机验收状态。

从 ZIP 解压后运行 `Sidelet.exe`。资料仍写到 `%APPDATA%\SideletMyGoLab`；build 43 首次启动会将原生 SQLite v1 升级到 v2，增加新建草稿表。升级前退出并备份完整资料，build 42 及更早版本不能直接读取升级后的库；回退需升级前备份并保留新版期间的数据。测试请按包内说明使用 `-data-dir` 指向独立目录。

## 图标与版本

版本号和构建号继续读取现有的 `packaging/macos/app.json`；这个历史路径现在同时供两平台打包使用。产品版本为 `0.2.0`，四段文件版本为 `0.2.0.43`。构建号超出 Windows 单段 16 位范围时拒绝构建，避免静默截断。

品牌图标唯一源文件为 `cmd/sidelet/design/sidebar-avatar.svg`（与悬浮精灵一致的完整造型）。修改后在仓库根目录运行以下命令，同步生成应用 PNG、Windows ICO、macOS ICNS、托盘 PNG 和官网 SVG；所有尺寸都保留同一造型：

```sh
go run scripts/prepare-icons.go
```

打包工具读取 `assets/sidelet.ico`，嵌入 16 / 20 / 24 / 32 / 40 / 48 / 64 / 128 / 256px 图标，所有尺寸均使用完整便签精灵。资源编号为 MyGo Windows 后端使用的 1；安装器复用同一图标。macOS 打包读取 `assets/sidelet.icns`。VERSIONINFO 包含产品名、公司名 Sidelet、内部名、原始文件名、版本及兼容的 MyGo 应用标识；标记为预发布。清单使用普通用户权限、Windows 10 兼容声明、Common Controls v6、PerMonitorV2 DPI 感知。系统中的最终显示与缩放仍需实机验证。

## 构建依赖与失败处理

`tools/windows-release` 是独立 Go 模块，固定 `github.com/tc-hib/winres v0.3.1` 生成 COFF 资源，只在构建机器上运行。根应用模块不依赖它，运行时不增加此库或 WebView。应用采用无 cgo、去除调试符号的 GUI 构建，并启用 MyGo 的 production 标志。

工具以独占创建方式生成当前架构的临时 `.syso`，正常结束或返回错误时清理；已有同名资源时拒绝覆盖。若进程被强制终止，确认没有构建正在运行后，可手动删除相应的 `cmd/sidelet/sidelet_windows_<arch>.syso` 再重试。

应用编译到临时路径，经架构、GUI 类型和完整资源字节比对通过后才替换已有 EXE。ZIP 从固定文件列表生成，重新读取每个文件并校验 CRC 和完整内容；不会遍历资料目录。校验文件在 ZIP 完成验证后生成。普通失败不会用不合格 EXE 覆盖之前的成功构建。

## 本轮验证

- 共用打包工具测试与 race：通过；覆盖构建失败保留旧 EXE、临时资源清理、资源所有权、版本溢出和 ZIP 内容异常拒绝。
- Go vet：应用的 Windows amd64 / arm64 目标和打包工具通过。
- 原有桌面及 internal 单元测试：通过。
- amd64 / arm64 EXE 与 ZIP 生成：通过；从最终 EXE 读取资源并逐字节核对图标、清单、VERSIONINFO；ZIP 重新读取内容及 SHA-256 校验通过。
- Windows 原生打包入口、文件版本 API、打包 EXE 的正常启动 / 退出与 SQLite 保存恢复：两种架构的 CI 均通过。
- build 36 增加窗口透明区域、被动焦点、侧栏 / 卡片交接、拖动及编辑恢复的自动检查，范围和最终结果见 [输入适配记录](WINDOWS_INPUT.md)。
- build 38 两架构新增 10 项系统鼠标 / 键盘跨进程检查通过，原有登录启动、安装器和持久化检查继续通过；最终包已下载并重新核对校验值，见 [跨进程输入记录](WINDOWS_CROSS_PROCESS_INPUT.md)。
- build 39 两架构新增 9 项实际通知提交、SQLite 回执、重启去重及完成 / 推迟撤回请求检查通过；原有输入、登录启动和安装器检查继续通过。最终包已下载复核，横幅 / 点击 / 声音仍待验收，见 [通知适配记录](WINDOWS_NOTIFICATIONS.md)。同轮改善通知提示、设置页图文对齐和快捷键展示，见 [UI 记录](REMINDER_UI_20261023.md)。
- build 40 两架构通过新增草稿保护 / 备注快捷保存回归及原有原生 CI 检查，源码 `12be29f`；见 [表单体验记录](TASK_FORM_20261023.md)。
- build 41 两架构主界面布局回归通过，源码 `97b672f`。build 42 的补充构建两架构全部通过，源码 `013ba83`；该构建产物未下载另作本地复验。build 43 草稿持久化的验证见 [草稿记录](NEW_TASK_DRAFT_20261023.md) 和 [CI 记录](WINDOWS_CI.md)。
- Windows 实际图标显示、鼠标、输入法、通知等交互及独立普通账户登录：仍待实机验收。受限 Medium 进程的权限检查单独记录在 [非管理员进程验收](WINDOWS_NONADMIN_20261023.md)。

build 43 的旧库迁移和安装 / 重装草稿保留补充验证见 [升级验收记录](WINDOWS_UPGRADE_20261023.md)。

2026-10-24 两架构的受限 Medium 权限安装、SQLite 写入 / 迁移及草稿重启恢复已通过；最终测试包已下载并校验。实际通过范围和 ARM64 提示框偶发失败记录见 [非管理员进程验收](WINDOWS_NONADMIN_20261023.md)。

`bash scripts/test-regression.sh` 已包含独立打包工具模块的测试与 vet。Windows 当前提醒能力边界见 [通知适配记录](WINDOWS_NOTIFICATIONS.md)。按用户安装器已接入，构建与生命周期验证见 [安装器记录](WINDOWS_INSTALLER.md)。仍未做 Authenticode 签名或发布官网的下载链接。

## 依据

- Microsoft VERSIONINFO：<https://learn.microsoft.com/en-us/windows/win32/menurc/versioninfo-resource>。
- 构建工具库及其 COFF / EXE 资源 API：<https://github.com/tc-hib/winres/tree/v0.3.1>。
