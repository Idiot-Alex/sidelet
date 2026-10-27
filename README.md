# Sidelet

Sidelet 是使用 Go + MyGo 构建的原生桌面任务应用。桌面端已切换为 MyGo，Wails / WebView / Svelte 桌面实现已移除；`website/` 是独立的产品官网。

## 运行与构建

需要 Go 1.27.1；macOS 打包另需 Xcode Command Line Tools。桌面构建无需 Node.js。

```sh
bash scripts/go.sh run ./cmd/sidelet
bash scripts/build-macos.sh arm64
bash scripts/package-macos.sh arm64
open build/bin/Sidelet.app
bash scripts/build-windows.sh amd64
bash scripts/package-windows.sh amd64
bash scripts/test-regression.sh
```

macOS 产物为 `build/bin/Sidelet.app`；Windows 产物为 `build/bin/Sidelet-amd64.exe`（也支持 arm64 交叉编译，已通过 x64 / ARM64 原生 CI，完整交互仍待实机验收）。本地 macOS 包为 ad-hoc 签名，未公证。Windows 构建会嵌入 Sidelet 多尺寸图标、文件版本、产品信息和运行清单；`package-windows.sh` / `package-windows.ps1` 生成 ZIP 与 SHA-256，命令和实机验收见 [Windows 打包记录](docs/mygo/WINDOWS_PACKAGE.md)。

测试可用 `-memory -main`，仅生成三条内存任务；`-data-dir <目录>` 指定独立 SQLite 资料目录。不要把测试指向日常数据。

当前打包构建号为 46。macOS 安装包名包含版本与构建号，例如 `Sidelet-0.2.0-build46-local-arm64.dmg`；打包脚本只读挂载 DMG，检查包内版本、架构和签名后生成 `.dmg.json` 校验记录及 `.sha256`。官网构建校验记录与实际包哈希一致后才提供下载；旧包不会被标记成新构建。构建命令和校验边界见 [macOS 打包与官网下载校验](docs/mygo/MACOS_PACKAGE_20261026.md)。

## 数据与应用身份

为延续已有 MyGo 任务和授权，本轮保留 bundle ID `io.sidelet.mygo-lab` 与默认资料目录 `SideletMyGoLab`，显示名称为 Sidelet。macOS 下默认目录为 `~/Library/Application Support/SideletMyGoLab`。这两个旧名称是兼容标识，不代表仍存在 WebView 模式。

旧 WebView 的 SQLite 数据不会删除、覆盖或自动迁移。build 30 起支持显式离线导入：先备份旧库，再向空的 MyGo 资料事务写入任务与迁移凭据，重复执行会跳过。命令与保护规则见 [旧版迁移](docs/mygo/LEGACY_MIGRATION.md)。MyGo 使用独立的原生任务表；旧版仍可从 Git 历史找回。

当前数据库为资料目录中的 `sidelet-native.sqlite3`，任务逐条存储，设置、排序和提醒回执与任务在同一事务提交。首次启动自动导入同目录的 `profile.json`（支持版本 1 / 2），原 JSON 保留为备份；之后只读取 SQLite，不会重复导入。数据库损坏或版本不支持时拒绝启动，不回退到旧备份覆盖新数据。备份前退出应用，并复制整个资料目录（包括可能存在的 WAL 文件）。

## 结构

- `cmd/sidelet/`：原生应用、窗口、输入适配、SQLite 持久化、提醒、主题与测试。
- `cmd/sidelet/design/web-ui.json`：继承原界面的主题与图标资产，现为原生设计的独立来源，文件名仅为历史名称。
- `internal/`：任务、设置、提醒、排序、快速添加、导出与旧存储兼容代码。
- `website/`：产品官网，使用 `npm run dev:website` / `npm run build:website`。
- `docs/mygo/`、`docs/phase-*`：历史实现与验收证据；旧路径和旧构建命令仅描述当时状态。

## 当前边界

已具备任务管理、三主题、侧栏与卡片、快速添加、macOS 提醒和导出；Windows 通知适配已接入，能力边界与待验收项见 [Windows 通知记录](docs/mygo/WINDOWS_NOTIFICATIONS.md)。切换主线不代表已完成全部发布验收：Windows 实机、真实鼠标跨窗、多屏 / Spaces、中文输入法候选、部分键盘操作和多任务组仍需后续处理；旧版迁移目前支持 schema 2、单任务组、空目标。

SQLite 实现与导入规则见 [存储记录](docs/mygo/SQLITE.md)。

最近内存短测见 [测量记录](docs/mygo/MEMORY_20261015.md)。

Windows CI 仅手动触发，push / pull request 不会自动运行。需要时在仓库 Actions → Windows → Run workflow 启动，两架构测试与产物、运行范围和验收边界见 [CI 记录](docs/mygo/WINDOWS_CI.md)。

Windows 侧栏透明命中、被动焦点、卡片交接和拖动适配见 [输入适配记录](docs/mygo/WINDOWS_INPUT.md)。

Windows 后台输入、光标恢复及快捷添加的系统事件验证见 [跨进程输入记录](docs/mygo/WINDOWS_CROSS_PROCESS_INPUT.md)。

提醒状态、设置页与三主题的本轮界面整理见 [UI 记录](docs/mygo/REMINDER_UI_20261023.md)。

主窗口草稿保护、备注快捷保存和折叠选项摘要见 [表单体验记录](docs/mygo/TASK_FORM_20261023.md)。

build 43 起，新建任务草稿自动保存至 SQLite，关闭窗口或重启后可继续填写；保存时机、失败重试和数据库升级边界见 [草稿持久化记录](docs/mygo/NEW_TASK_DRAFT_20261023.md)。

build 44 起，任务页默认单列，新建 / 编辑时展开编辑区；收起保留草稿。任务状态合并、设置页与浮动卡片整理见 [主界面布局记录](docs/mygo/MAIN_LAYOUT_20261023.md)。

主窗口固定使用实色背景和细分隔任务列表，不提供背景透视。桌面位置、标签密度与“悬浮任务半透明”在设置的“桌面任务”分类中调整，透明默认关闭，开关保存在本机 SQLite 中。Windows 浮动卡片使用无模糊透明合成，macOS 使用原生磨砂材质；系统降低透明度、提高对比度或不支持原生材质时使用实色背景，侧边标签的空白区域仍完全透明。

设置分为“外观、桌面任务、通知与启动、数据与快捷键”；“悬浮任务半透明”位于“桌面任务”。侧边优先级通过颜色与不同条数区分，展开后显示重要 / 紧急文字。build 46 收录这两次 UI 更新，详见 [最新布局说明](docs/mygo/MAIN_LAYOUT_20261023.md#2026-10-26build-46-收录最新界面行为)。

侧栏默认显示原创动漫头像，轻点可打开主界面、快速添加、设置，或隐藏 / 显示桌面任务；拖动头像可独立调整位置，不影响任务条，松开自动保存头像位置，Esc 或失去焦点取消拖动。没有任务、隐藏任务时仍可使用头像入口；整理模式暂时隐藏头像以免影响拖动。可在“设置 → 桌面任务 → 侧栏助手”关闭，选择会保存。头像素材嵌入程序，无需联网下载。

任务时间只需设置“预计完成时间”和“到时提醒”，主界面与快速添加均可开启提醒。侧栏卡片保留“完成”和“编辑”，不再提供单项任务的“稍后 / 暂时隐藏”；固定任务持续显示，直到完成或取消固定。启动时会恢复旧版暂时隐藏的任务，保留原预计完成时间和固定状态，已发送的提醒不会因恢复而重复发送。

macOS 浮动卡片点击后空白的输入层防遮挡修复见 [卡片窗口层级记录](docs/mygo/MACOS_CARD_ORDER_20261023.md)。

MyGo SDK 当前固定为 `v0.5.1`；迁移、验证与内存对比见 [升级记录](docs/mygo/UPGRADE_051.md)。

Windows 按用户安装包的构建、升级和卸载保护见 [安装器记录](docs/mygo/WINDOWS_INSTALLER.md)。

Windows 打包程序的旧数据库升级、安装 / 重装后的草稿保留与回退说明见 [升级验收记录](docs/mygo/WINDOWS_UPGRADE_20261023.md)。

Windows 受限 Medium 权限下的安装、卸载和 SQLite 恢复检查见 [非管理员进程验收](docs/mygo/WINDOWS_NONADMIN_20261023.md)。

Windows 登录启动、系统禁用状态与卸载清理见 [登录启动记录](docs/mygo/WINDOWS_LOGIN_STARTUP.md)。
