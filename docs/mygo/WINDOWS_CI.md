# Windows 自动验证

更新：2026-10-23。工作流：`.github/workflows/windows.yml`。

主线桌面源码、构建脚本、依赖、资产或打包配置改变时，在 push / pull request 上运行；也可在 Actions 中手动运行。两项任务分别使用 Windows x64 和 Windows ARM64 主机，Go 版本取自根 `go.mod`。官方 Action 固定到提交 SHA，仓库 token 仅有读取内容权限；不会自动发布 Release 或修改官网。

`scripts/test-windows-ci.ps1` 用 PowerShell 7 执行以下检查：

1. 确认 Go 主机架构与测试架构一致，避免把交叉编译当作运行验证。
2. 执行桌面 / internal 测试、vet，以及独立发布工具模块的测试和 vet。
3. 调用现有 `package-windows.ps1`，生成并验证 ZIP 和资源。
4. 比对 ZIP 的 SHA-256，解压后用 Windows 文件版本 API 检查产品和文件版本。
5. 对 ZIP 中的实际 EXE 执行 `TestPackagedApplicationSmoke`：启动、自动退出，再次启动、退出；每次确认到达原生主窗口启动阶段、资料标识正确、无存储错误。
6. 从 SQLite 直接读取状态，验证中文、备注、重要程度、固定状态等字段恢复；由实际应用启动清理过期临时任务并保存删除标记；退出后资料锁可重新获得。
7. build 36 起，运行 ZIP 内 EXE 的 Windows 侧栏原生输入检查；窗口区域、焦点与拖动的覆盖及限制见 [输入适配记录](WINDOWS_INPUT.md)。
8. 编译按用户安装器；在独立临时主机验证安装、运行中拦截、合成低版本升级、拒绝降级、同版本覆盖、卸载保留资料及重装恢复。具体夹具与验证边界见 [安装器记录](WINDOWS_INSTALLER.md)。
9. build 37 起，验证登录项真实注册表读写与 SQLite 失败恢复、实际打包 EXE 的四种登录/手动启动呈现，以及安装器保留/清理登录项；见 [登录启动记录](WINDOWS_LOGIN_STARTUP.md)。
10. build 38 起，使用两个独立进程及系统鼠标/键盘事件验证后台输入、光标保持、卡片保存/取消和正式全局快捷添加；见 [跨进程输入记录](WINDOWS_CROSS_PROCESS_INPUT.md)。
11. build 39 起，运行实际 EXE 的系统通知提交、回执落库、正常重启去重、卡片完成 / 推迟撤回检查；横幅及 UI Automation 激活单独记录，见 [通知适配记录](WINDOWS_NOTIFICATIONS.md)。
12. build 43 补充验证打包 EXE 将旧 SQLite v1 升级到 v2，任务 / 设置 / 提醒回执保持，迁移后新建草稿可以保存并重启恢复；安装器各次启动也检查完整草稿，见 [旧库升级验收](WINDOWS_UPGRADE_20261023.md)。

测试只创建独立合成资料。启动冒烟通过 `-quit-after` 控制正常退出，单次进程另有 45 秒超时；跨进程输入测试使用 90 秒总超时，结束只清理自己启动的两个进程。默认测试跳过可执行文件冒烟；显式设置 `SIDELET_SMOKE_EXE` 才运行，跨进程系统输入还要求 GitHub 托管临时主机。`SIDELET_SMOKE_OUTPUT` 可指定诊断父目录，测试始终在其下创建随机新目录，不复用现有资料。

本机复用测试逻辑的命令（原生程序需要相应桌面运行环境）：

```sh
SIDELET_SMOKE_EXE="$PWD/build/bin/Sidelet.app/Contents/MacOS/sidelet" \
SIDELET_SMOKE_OUTPUT="$PWD/build/results/windows-ci-local" \
bash scripts/go.sh test -count=1 -timeout=2m ./cmd/sidelet -run '^TestPackagedApplicationSmoke$' -v
```

成功时保留两架构 ZIP、安装器和校验文件；无论成功与否，都尝试保存本轮合成诊断及测试日志，保留期 7 天。诊断中包括本轮合成 SQLite 资料，不包含日常任务。

## 验证状态与边界

本地桌面 / internal 测试、Go vet、工作流 YAML 解析通过；Mac 上打包 EXE 的两次启动 / 退出、SQLite 清理写入及恢复检查通过。首轮原生 Windows x64 / ARM64 已执行，均发现 SQLite URI 把 `C:` 当成主机名的问题，导致资料库无法打开；另有两处 Unix 文件模式断言不适用于 Windows。现已统一修正原生库、旧库及迁移读取路径的盘符格式，并让权限断言按平台执行，新增盘符 / Unicode / 特殊字符路径回归。build 34 的[原生 Windows 复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39103900224)已全部通过：x64 3 分 23 秒，ARM64 3 分 47 秒；测试、vet、PowerShell 打包、Windows 版本资源 API、ZIP 校验及打包 EXE 的两次启动 / 退出与 SQLite 持久化恢复均通过。验证源码提交为 `97c161f`。两架构包及校验文件已从 CI 下载、复验并存入本地 `build/releases/`。

build 35 的 [安装器复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39642384601) 已通过两架构全部步骤，验证源码 `83d0bb2`。包含按用户安装、运行中升级 / 卸载拦截、合成低版本升级、同版本覆盖、拒绝降级，以及卸载保留 SQLite、重新安装恢复中文任务。两架构 ZIP、安装器和校验文件已下载并复验。

build 36 的 [最终复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39809334284) 两架构全部通过，验证源码 `12865c6`。新增 18 项 Win32 窗口区域、悬停、提示框、焦点、拖动和清理检查；原有持久化、打包与安装器生命周期检查继续通过。ARM64 临时主机先处理已知的首次登录 Shell 窗口干扰，具体步骤与限制见 [输入适配记录](WINDOWS_INPUT.md)。

build 37 的 [登录启动复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39817132621) 两架构全部通过，验证源码 `b1fa189`：x64 2 分 52 秒，ARM64 4 分 7 秒。新增真实注册表专项测试、打包 EXE 四种启动呈现和安装器启动项保留/清理检查；原有 18 项输入和持久化测试继续通过。最终 ZIP 和安装器均下载并校验，证据见 `build/results/windows-login-39817132621/verified.json`。真实注销/登录不在本次 CI 范围内。

build 38 的 [跨进程输入复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39819946040) 两架构全部通过，验证源码 `641534f`：x64 3 分 15 秒，ARM64 4 分钟。每个架构新增 10 项独立进程系统输入检查，确认侧栏 / 只读卡片不打断后台输入，编辑与快速添加保存 / 取消恢复原光标，失焦保留快速添加草稿。原有测试和安装器生命周期继续通过；最终包已下载复验，证据见 `build/results/windows-focus-39819946040/verified.json`。

build 39 的 [通知链路复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39827518998) 两架构全部通过，验证源码 `3751d1c`：x64 3 分 24 秒，ARM64 5 分 27 秒。每个架构通过 9 项实际通知提交 / SQLite 回执 / 重启去重 / 完成和推迟撤回请求检查，以及 19 项窗口输入、10 项跨进程输入、4 种登录启动和完整安装器生命周期检查。x64 未捕获可见通知横幅；ARM64 横幅探测进程未成功运行，点击和声音均未验收。最终包已下载并校验，证据见 `build/results/windows-notifications-39827518998/verified.json`。

build 40 的 [表单体验复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39831015862) 两架构全部通过，验证源码 `12be29f`：x64 3 分 29 秒，ARM64 4 分 43 秒。新增草稿恢复、重复选择、部分日期及备注快捷保存测试通过；原有窗口输入、跨进程输入、提醒、登录启动与安装器生命周期检查继续通过。最终产物校验记录在 `build/results/windows-form-39831015862/verified.json`，变更说明见 [表单体验记录](TASK_FORM_20261023.md)。

build 41 的 [主界面布局复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39834104048) 两架构任务全部通过，源码 `97b672f`：x64 2 分 52 秒，ARM64 4 分 44 秒。build 42 增加 macOS 卡片输入层防遮挡；对应 [Windows 补充构建](https://github.com/Idiot-Alex/sidelet/actions/runs/39836025277) 两架构全部通过，源码 `013ba83`：x64 3 分 9 秒，ARM64 4 分 53 秒，包含打包运行与安装器生命周期。本轮未下载该构建产物另作本地复验，见 [卡片修复记录](MACOS_CARD_ORDER_20261023.md)。

build 43 的 [草稿持久化复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39967751427) 两架构全部通过，源码 `ef5a0e0`：x64 5 分 10 秒，ARM64 5 分 23 秒。新增 SQLite 草稿、失败 / 重试与事务回滚测试通过；最终包确认主窗口从 SQLite 恢复草稿、临时任务清理后仍保留、两次退出后资料一致且锁释放。既有输入、提醒、登录启动及安装器生命周期继续通过。后续已下载并复核两架构产物，证据位于 `build/results/windows-build43-39967751427/verified.json`，见 [草稿记录](NEW_TASK_DRAFT_20261023.md)。

同为 build 43 的 [发布验收补充](https://github.com/Idiot-Alex/sidelet/actions/runs/39975853709) 两架构全部通过，源码 `b3f1779`：x64 4 分 52 秒，ARM64 5 分 23 秒。新增打包程序升级 SQLite v1、保留回执与迁移后草稿恢复检查；安装器各次启动比对完整草稿。前一轮 ARM64 暴露的测试时间戳依赖已改为可控时间验证，应用运行逻辑不变。最新 ZIP / 安装器已下载到 `build/releases/` 并复验，记录见 `build/results/windows-upgrade-39975853709/verified.json` 和 [升级验收说明](WINDOWS_UPGRADE_20261023.md)。

CI 检查的是托管 Windows 环境中的自动测试，不是完整人工验收。物理键鼠、第三方应用特殊控件、多屏、输入法候选、通知横幅 / 声音 / 点击仍需单独测试。托管 Windows runner 以管理员身份运行，不能代表普通用户权限下已经通过。清单见 [Windows 打包记录](WINDOWS_PACKAGE.md)。

官方依据：

- <https://docs.github.com/en/actions/reference/runners/github-hosted-runners>
- <https://github.com/actions/setup-go>
- <https://github.com/actions/upload-artifact>
