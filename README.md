# Sidelet

Sidelet 是使用 Go + MyGo 构建的原生桌面任务应用。桌面端已切换为 MyGo，Wails / WebView / Svelte 桌面实现已移除；`website/` 是独立的产品官网。

## 运行与构建

需要 Go 1.27.1；macOS 打包另需 Xcode Command Line Tools。桌面构建无需 Node.js。

```sh
bash scripts/go.sh run ./cmd/sidelet
bash scripts/build-macos.sh arm64
open build/bin/Sidelet.app
bash scripts/build-windows.sh amd64
bash scripts/package-windows.sh amd64
bash scripts/test-regression.sh
```

macOS 产物为 `build/bin/Sidelet.app`；Windows 产物为 `build/bin/Sidelet-amd64.exe`（也支持 arm64 交叉编译，已通过 x64 / ARM64 原生 CI，完整交互仍待实机验收）。本地 macOS 包为 ad-hoc 签名，未公证。Windows 构建会嵌入 Sidelet 多尺寸图标、文件版本、产品信息和运行清单；`package-windows.sh` / `package-windows.ps1` 生成 ZIP 与 SHA-256，命令和实机验收见 [Windows 打包记录](docs/mygo/WINDOWS_PACKAGE.md)。

测试可用 `-memory -main`，仅生成三条内存任务；`-data-dir <目录>` 指定独立 SQLite 资料目录。不要把测试指向日常数据。

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

Windows 自动测试与产物位于仓库 Actions，运行范围和验收边界见 [CI 记录](docs/mygo/WINDOWS_CI.md)。

MyGo SDK 当前固定为 `v0.5.1`；迁移、验证与内存对比见 [升级记录](docs/mygo/UPGRADE_051.md)。
