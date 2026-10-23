# Windows 非管理员进程验收

2026-10-23 开始，2026-10-24 更新；版本仍为 0.2.0 build 43。本轮只修改 CI 验收工具和脚本，不修改正式应用，也不将测试工具装入发布包。

## 最终结果

[Windows CI 39988133295](https://github.com/Idiot-Alex/sidelet/actions/runs/39988133295) 最终成功，验证源码 `aa88e85ded38f9cae26bc8200ed599ce7d947407`。x64 保留第一次尝试的成功结果（5 分 14 秒），ARM64 第三次尝试通过（6 分 50 秒）；失败尝试及限制见下文。

两架构均完成非提升 Medium 权限下的 SQLite 写入 / 旧库升级 / 重启恢复和完整安装生命周期。下载后分别核对 14 份令牌报告（含 9 次安装 / 卸载进程、3 次实际已安装应用启动）、系统注册表写权限拒绝、5 项旧库升级检查及三次完整草稿恢复。

最终 ZIP、安装器、SHA-256 已下载并复验固定 ZIP 清单 / CRC、EXE 架构 / GUI 类型、版本及干净源码信息，存入 `build/releases/`。证据目录为 `build/results/windows-nonadmin-39988133295/`，其中 `ci-run.json` 记录最终 CI，`nonadmin-verification.json` 记录权限和数据恢复核对，`verified.json` 记录包校验；前两次 ARM64 失败日志也保留在该目录。

ARM64 的提示框区域检查曾在第二次尝试失败，第三次通过，尚未稳定复现或确定原因。该项继续跟踪，不能把本轮成功解读为已排除全部交互稳定性问题。

## 方法与范围

GitHub 托管 Windows runner 使用管理员账户且关闭 UAC，仅有 `asInvoker` / `RequestExecutionLevel user` 声明不能证明普通权限运行。新增 `tools/windows-ci` 从当前进程创建受限令牌：`CreateRestrictedToken` 的 `LUA_TOKEN | DISABLE_MAX_PRIVILEGE`、管理员 SID 设为 deny-only，完整性级别设为 Medium。再用该令牌启动 PowerShell 验收脚本。

受限令牌的新对象默认所有者设为当前用户，默认 DACL 仅授予当前用户和 SYSTEM 访问，以便创建自己的输出管道与进程对象；不更改现有对象 ACL。权限报告保留实际默认 DACL，前置回归验证子进程在降权状态创建并读写自己的管道。

不会新建账户、修改 UAC、授予管理员权限或改变系统安全策略；这是一组受限进程功能测试，不是运行不可信程序的安全沙箱。工具与生命周期脚本都要求 GitHub 托管临时 Windows 主机。实际安装、注册表和资料路径仍有“已有 Sidelet 状态则拒绝”的保护。

检查点：

- PowerShell 的真实令牌必须是非提升、Medium，管理员组不存在或仅 deny-only；除 `SeChangeNotifyPrivilege` 外不得有启用的权限。任何查询失败也判定失败。
- 尝试以可写方式打开既有 `HKLM\SOFTWARE` 必须拒绝；不创建或修改系统注册表内容。
- 重新执行实际打包 EXE 的 SQLite 冒烟和 v1 → v2 升级测试。夹具写入、实际程序启动清理、迁移、草稿持久化与重启恢复都在受限进程链中运行。
- 完整安装器生命周期在同一权限下执行。安装器 / 卸载器先挂起创建，读取其真实令牌通过后才恢复运行，避免短进程退出造成取证竞态；60 秒超时只终止本次创建的进程。
- 三次已安装应用启动分别读取实际 PID 的令牌，并验证任务和完整草稿恢复。保留升级 / 卸载运行拦截、低版本拒绝、修复、HKCU 注册、快捷方式、登录项和无关文件保护检查。

结果保存到 `build/results/windows-ci-<arch>/nonadmin/`，安装器各进程令牌记录位于同级 `installer-*/`。仅全部通过后生成 `nonadmin/result.json`；失败不降级为管理员重试，也不跳过权限断言。

## 状态与限制

本地 x64 / ARM64 交叉编译及 vet 通过。首轮 [39980432683](https://github.com/Idiot-Alex/sidelet/actions/runs/39980432683) 和第二轮 [39981321508](https://github.com/Idiot-Alex/sidelet/actions/runs/39981321508) 的应用回归通过，但两架构权限工具均在 `TokenElevation` 查询处失败：该固定结构需要精确的 4 字节长度，零长度探测和较大的通用缓冲区都会返回 `ERROR_BAD_LENGTH`。已改为固定结构读取，保留查询失败即失败的规则，并新增实际令牌读取 / 创建回归；工具测试已前置到 CI 开头。

第三轮 [39982220098](https://github.com/Idiot-Alex/sidelet/actions/runs/39982220098) 已通过真实令牌读取 / 创建回归，但降权 PowerShell 缺少 CI 环境标识，被临时主机保护拒绝。Go 在指定令牌且 `Cmd.Env` 为空时会生成账户登录环境；现显式继承已验证的父进程环境，保留所有环境保护，新增真实降权子进程检查。

第四轮 [39983172368](https://github.com/Idiot-Alex/sidelet/actions/runs/39983172368) 两架构通过环境与实际降权子进程检查，系统注册表也确实拒绝了写访问；但 PowerShell 包装后的异常没有被测试识别。已将负向检查移至 Go，直接断言 Win32 `ERROR_ACCESS_DENIED`，任何其他错误仍失败；该检查同时加入前置降权子进程回归。

第五轮 [39984306685](https://github.com/Idiot-Alex/sidelet/actions/runs/39984306685) 两架构通过令牌与注册表拒绝检查，但带输出重定向的测试程序启动返回拒绝访问。针对过滤后的管理员令牌可能保留管理员对象默认权限的问题，设置上述当前用户对象默认值，并新增真实管道回归及可执行文件 ACL 诊断；尚未将该失败归因于应用。

第六轮 [39986016755](https://github.com/Idiot-Alex/sidelet/actions/runs/39986016755) 的 ARM64 已通过管道回归，实际测试程序可以启动；未加引号的 Go 点号参数被 PowerShell 拆分为 `-test`，因此未执行数据库测试。已统一改为字面参数数组。x64 随新提交取消，不记为通过。

第七轮 [39987028505](https://github.com/Idiot-Alex/sidelet/actions/runs/39987028505) ARM64 全部通过；x64 通过全部应用、数据库和安装生命周期检查，最后向 runner 受保护的摘要文件写入时被拒绝。下载诊断后，每架构均独立核对 14 份真实令牌报告、9 次安装 / 卸载进程、5 项旧库升级检查及三次完整草稿恢复；x64 未生成最终成功标记，因此整轮仍记失败。汇总现由原 CI 进程在降权套件成功退出后执行，不改变 runner 文件权限。该轮诊断保存在 `build/results/windows-nonadmin-39987028505/partial-verification.json`。

第八轮 [39988133295](https://github.com/Idiot-Alex/sidelet/actions/runs/39988133295) 首次尝试的 x64 全部通过（5 分 14 秒）；ARM64 在既有安装器构建脚本清理临时 `makensis.exe` 时返回拒绝访问，尚未进入降权套件。安装器已构建并校验，此次仅重跑失败的 ARM64 任务，保留同源码已通过的 x64 结果。

该运行第二次尝试的 ARM64 在原有 `tooltip-region` 诊断失败：800ms 后提示区域仍为零，尚未进入降权套件。保留失败日志，未放宽断言；第三次尝试仅重跑 ARM64，在新的托管桌面复查。此结果不计为权限验收通过，也不据此断言悬浮提示已无稳定性问题。

第三次尝试 ARM64 通过原有提示框区域检查与全部降权验收，最终状态成功。没有为重跑调整应用代码、放宽断言或跳过失败项。

这证明的是同一个 runner 账户下的受限 Medium 进程，不是独立普通账户登录。独立账户 ACL、UAC / SmartScreen 提示、安装向导交互、真实登录、多屏、输入法和通知横幅仍需实机验收；打包元数据中的 `runtimeAcceptance: pending` 保持不变。

官方依据：

- [GitHub 托管 runner 权限](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#administrative-privileges)
- [CreateRestrictedToken](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-createrestrictedtoken)
- [CreateProcessAsUserW](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessasuserw)
- [Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
- [匿名管道的默认安全描述符](https://learn.microsoft.com/en-us/windows/win32/ipc/anonymous-pipe-security-and-access-rights)
- [TOKEN_DEFAULT_DACL](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-token_default_dacl)
