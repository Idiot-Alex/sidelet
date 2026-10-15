# MyGo 主线切换

2026-10-15，按用户决定停止维护 WebView 桌面版本。

- 原 `experiments/mygo-sidebar` 原生代码移入 `cmd/sidelet`，合并至根 Go module；命令行默认选择完整原生应用。
- 删除 `cmd/spike`、桌面 `frontend`、旧平台宿主及专属测试工具；删除 MyGo 混合 Web 窗口、内嵌 HTML 与 Wails 依赖。
- 主题 / 图标 JSON 成为独立设计资产，构建不再读取 Svelte 源码。历史记录移动到 `docs/mygo`，其中旧命令及路径仅作历史证据。
- 官网互动组件独立保存在 `website/src/demo`，不使用 Wails runtime 或桌面通信；静态标记不提供原生整组移动功能。
- 默认 MyGo 资料目录与 `io.sidelet.mygo-lab` 标识保持兼容，应用名称为 Sidelet，版本 0.2.0 / build 26。旧 SQLite 数据不迁移、不删除；旧存储库仅保留兼容与测试用途，未链接到当前原生程序。
- 旧版本已安装应用未自动卸载；请退出旧版后使用新包。主线切换并不表示所有平台和功能已验收。

验证：`go test` / race / vet 覆盖 `cmd/sidelet` 与全部 `internal`；macOS arm64 构建及严格签名检查通过；Windows amd64 / arm64 交叉构建通过（未运行）；官网 Svelte 检查零错误、零警告，Vite 构建通过。检查当前应用依赖树无 Wails / SQLite。

Mac 启动检查使用 `build/bin/Sidelet.app -memory -main`，未传 `-native-main`，日志确认为 native 渲染器，Computer Use 读取到三条合成任务及完整主窗口。日志在 `build/results/mygo-cutover-20261015/`。该测试不使用用户资料或更改系统授权。
