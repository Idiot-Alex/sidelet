# Windows 登录启动

2026-10-23，0.2.0 build 37。Windows x64 / ARM64 接入设置页“登录后启动 Sidelet”；macOS 仍使用原有 SMAppService。

## 用户行为

- 默认不注册；安装、升级、启动和检查状态都不会自动开启。
- 开关读取 Windows 的实际注册和禁用状态，区别“未开启”“已开启”“已在系统中禁用”“另一位置的 Sidelet”“无法读取”。SQLite 中的历史偏好不能覆盖系统状态。
- 在 Windows 设置或任务管理器禁用后，应用保持关闭状态。用户明确重新开启应用内开关时，才重新允许该启动项。“打开系统登录项”进入 Windows 设置的启动应用页。
- 登录时按“启动时显示主窗口”偏好显示；关闭时仅保留托盘和固定任务。首次资料的登录启动也不强制弹主窗口，普通首次手动打开仍显示主窗口。
- 便携版同样支持，但应先放到固定位置再开启；移动前在旧位置关闭。检测到同名启动项指向另一位置时不覆盖，提示用户回到原位置关闭。

## 注册与失败恢复

只管理当前用户 64 位注册表视图的 `Software\Microsoft\Windows\CurrentVersion\Run` 中 `Sidelet` 值。命令为带引号的 EXE 绝对路径，加 `--mygo-opened-at-login`；MyGo v0.5.1 消费该参数并通过 `WasOpenedAtLogin()` 报告，复用现有启动流程。不保存 `-data-dir`、`-output` 等诊断参数，真实登录始终读取默认 SQLite 资料。

路径支持空格、中文、非 BMP 字符；拒绝 NUL、双引号和超过 Run 命令长度限制的路径。不要求管理员权限，不增加计划任务或额外常驻进程。

Windows 的 `Explorer\StartupApproved\Run` 禁用信息不是公开稳定 API。当前只接受已知 12 字节记录，状态 2 / 6 视为允许、3 / 7 视为禁用；其他结构、类型或读取错误显示未知并禁用开关，不推测已开启。系统未来改变格式时需重新适配。显式开启时删除自己的禁用记录，不影响其他应用。

注册前保存命令与禁用记录快照。原生修改或 SQLite 保存失败时，恢复两个值及禁用时间戳；补偿失败明确提示不能确认恢复。若期间启动项被另一位置的应用接管，拒绝覆盖。查询、前台恢复和普通偏好保存不修改系统注册。

## 安装器

安装、同路径升级与修复均不修改登录项，保留用户的启用/禁用选择。卸载只在完整启动命令属于当前安装目录时移除 Run 和对应 StartupApproved 值；另一位置的便携版和其他应用值保留。卸载保留 SQLite，重新安装也不会根据旧偏好自动注册。

## 验证

- Windows 专用测试在随机私有 HKCU 测试子键验证真实注册表读写，不创建可被系统执行的启动项。覆盖 Unicode 命令、重复关闭、系统禁用只读、明确重新启用、未知记录、读取错误、另一安装位置保护、SQLite 失败恢复（含禁用时间戳）。
- 通用测试覆盖三主题的键盘开关、系统禁用提示和失败回退。
- ZIP 中实际 EXE 的 `TestPackagedWindowsLoginStartup` 覆盖首次登录隐藏、已有资料登录隐藏、偏好显示主窗口、首次手动打开，并检查固定任务保留。这是实际程序的参数与呈现测试，不冒充注销/再登录。
- 安装器 CI 在 GitHub 托管临时 Windows 主机验证首次安装不注册、升级/修复保留禁用记录、卸载清理自己的记录、另一位置与无关应用记录保留。检测到已有 Sidelet 安装、资料或启动项时拒绝测试。

本轮原生 CI 结果待执行。真正注销后重新登录、普通用户权限和任务管理器交互仍待实机验收；不改变当前 Mac 的登录项或日常数据。

## 依据

- [Microsoft Run / RunOnce 约定](https://learn.microsoft.com/en-us/windows/win32/setupapi/run-and-runonce-registry-keys)
- [Microsoft Windows 设置 URI（startupapps）](https://learn.microsoft.com/en-us/windows/apps/develop/launch/launch-settings#apps)
- [MyGo v0.5.1 Windows 登录实现](https://github.com/egoist/mygo/blob/v0.5.1/internal/windows/login.go) 与 [登录参数约定](https://github.com/egoist/mygo/blob/v0.5.1/login.go)
