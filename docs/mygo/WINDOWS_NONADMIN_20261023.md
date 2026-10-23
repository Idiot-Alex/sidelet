# Windows 非管理员进程验收

2026-10-23；版本仍为 0.2.0 build 43。本轮只修改 CI 验收工具和脚本，不修改正式应用，也不将测试工具装入发布包。

## 方法与范围

GitHub 托管 Windows runner 使用管理员账户且关闭 UAC，仅有 `asInvoker` / `RequestExecutionLevel user` 声明不能证明普通权限运行。新增 `tools/windows-ci` 从当前进程创建受限令牌：`CreateRestrictedToken` 的 `LUA_TOKEN | DISABLE_MAX_PRIVILEGE`、管理员 SID 设为 deny-only，完整性级别设为 Medium。再用该令牌启动 PowerShell 验收脚本。

不会新建账户、修改 UAC、授予权限或改变系统安全策略；这是一组受限进程功能测试，不是运行不可信程序的安全沙箱。工具与生命周期脚本都要求 GitHub 托管临时 Windows 主机。实际安装、注册表和资料路径仍有“已有 Sidelet 状态则拒绝”的保护。

检查点：

- PowerShell 的真实令牌必须是非提升、Medium，管理员组不存在或仅 deny-only；除 `SeChangeNotifyPrivilege` 外不得有启用的权限。任何查询失败也判定失败。
- 尝试以可写方式打开既有 `HKLM\SOFTWARE` 必须拒绝；不创建或修改系统注册表内容。
- 重新执行实际打包 EXE 的 SQLite 冒烟和 v1 → v2 升级测试。夹具写入、实际程序启动清理、迁移、草稿持久化与重启恢复都在受限进程链中运行。
- 完整安装器生命周期在同一权限下执行。安装器 / 卸载器先挂起创建，读取其真实令牌通过后才恢复运行，避免短进程退出造成取证竞态；60 秒超时只终止本次创建的进程。
- 三次已安装应用启动分别读取实际 PID 的令牌，并验证任务和完整草稿恢复。保留升级 / 卸载运行拦截、低版本拒绝、修复、HKCU 注册、快捷方式、登录项和无关文件保护检查。

结果保存到 `build/results/windows-ci-<arch>/nonadmin/`，安装器各进程令牌记录位于同级 `installer-*/`。仅全部通过后生成 `nonadmin/result.json`；失败不降级为管理员重试，也不跳过权限断言。

## 状态与限制

本地 x64 / ARM64 交叉编译及 vet 通过；真实 Windows 双架构 CI 待执行。实际结果后续记录在此处。

这证明的是同一个 runner 账户下的受限 Medium 进程，不是独立普通账户登录。独立账户 ACL、UAC / SmartScreen 提示、安装向导交互、真实登录、多屏、输入法和通知横幅仍需实机验收；打包元数据中的 `runtimeAcceptance: pending` 保持不变。

官方依据：

- [GitHub 托管 runner 权限](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#administrative-privileges)
- [CreateRestrictedToken](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
- [CreateProcessAsUserW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw)
- [Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
