# Phase 1：统一设置（macOS）

2026-10-05。范围：应用偏好、登录启动、通知入口。构建 4 新增 [外观主题](phase-1-ui-themes.md)，构建 5 新增 [Dock 显示开关](phase-1-macos-dock.md)。P0 尚未完成的直接鼠标拖动、多屏、Spaces 等项目保持原状态。

## 使用

主窗口的“设置”、菜单栏菜单的“设置…”或主窗口内的 Command + , 打开设置页。“返回我的任务”保留未提交的任务表单。

- 外观：精致 Mac（默认）、温暖纸色、深色石墨。主窗口、桌面标签和快速卡片同步，重启保留；旧设置文件缺少 appearance 时使用精致 Mac，原有偏好不变。
- 在 Dock 中显示：macOS 默认开启，切换即时生效并自动保存；关闭后保留当前设置窗口及菜单栏入口。点击 Dock 恢复关闭或最小化的主窗口，保留当前页面与未提交草稿；安静模式不受影响。
- 启动时显示主窗口：默认关闭；首次无设置且空库时显示引导窗口。显式 `-main` 始终显示。
- 登录 Mac 后启动：默认关闭，使用系统 `SMAppService.mainAppService` 注册。界面展示系统实际状态；需要批准时提示打开系统登录项。只在默认资料目录的 macOS 实例中提供注册开关，测试资料目录不会注册错误的登录实例。
- 新任务组默认边缘、密度：默认右侧、标准。只在创建新 Stack 时取值；已有 Stack 继续读取 SQLite，修改默认值不会移动任务或改变其密度。当前产品没有增加新任务组的独立入口，默认值在空布局初始化时生效。
- 通知状态：读取系统权限，支持主动申请或检查权限，并打开系统通知设置。仍需为具体任务勾选“提醒我”，且应用在后台运行。

## 保存与系统状态

按 Revision 4 第 39 节，偏好位于资料目录内的 `settings.json`。任务、现有 Stack 和提醒继续保存在 SQLite，数据库版本仍为 2。

```json
{
  "version": 1,
  "edge": { "defaultSide": "right", "defaultDensity": "normal" },
  "startup": { "enabled": false, "showMainWindow": false },
  "appearance": { "theme": "mac", "showDockIcon": true }
}
```

设置串行写入同目录的 0600 临时文件，写入和文件同步成功后替换旧文件；成功后才发布内存状态。写入失败回显旧值并提示。格式错误、未知版本或未知字段不会被静默覆盖，任务管理仍可运行，设置页提示修复原文件后重启。

`startup.enabled` 记录应用内的登录启动选择；macOS 状态为实际依据，用户在系统设置中修改后不会被应用启动时自动覆盖。注册成功而文件保存失败时尝试恢复原登录状态；恢复失败则明确提示并重新读取实际状态。系统注册与文件写入无法组成跨系统原子事务，进程在两者间崩溃时以 OS 状态为准，下一次主动操作再保存选择。

通知入口使用 macOS 通知面板 URL，失败时打开系统设置，页面保留手动导航说明。登录项入口使用 Apple 提供的 API。Windows 保留偏好文件和启动显示行为，登录项和通知系统入口暂不支持。

## 验证

自动化覆盖偏好重启恢复、写入失败不覆盖、无临时文件残留、损坏/未来版本保护、登录注册失败、保存失败补偿及补偿失败、默认值只用于新 Stack。

实机记录位于 `build/results/settings-native-20261005/`，最终验收结果见该目录 `report.json`。已实测自动保存、默认值不修改现有 Stack、目录不可写时恢复旧值、任务草稿保留、Command + ,、不带 `-main` 的显示/隐藏启动、macOS 登录项注册与注销，以及两个系统设置入口。正常资料目录仍为零任务，登录启动已恢复关闭。

最终检查：前端 19 项测试、存储 20 项、提醒引擎 3 项、设置 3 项测试通过；Go 竞态检查、vet、macOS arm64 构建与签名、Windows amd64/arm64 交叉编译通过。

未自动注销或重启用户电脑，因此注册状态验证不代表已经完成一次真实登录启动。当前是本地临时签名开发包，正式发布签名后的安装与登录流程仍需验收。

参考：[Apple SMAppService](https://developer.apple.com/documentation/servicemanagement/smappservice)、[打开系统登录项](https://developer.apple.com/documentation/servicemanagement/smappservice/opensystemsettingsloginitems())。
