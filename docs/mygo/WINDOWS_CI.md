# Windows 自动验证

更新：2026-10-22。工作流：`.github/workflows/windows.yml`。

主线桌面源码、构建脚本、依赖、资产或打包配置改变时，在 push / pull request 上运行；也可在 Actions 中手动运行。两项任务分别使用 Windows x64 和 Windows ARM64 主机，Go 版本取自根 `go.mod`。官方 Action 固定到提交 SHA，仓库 token 仅有读取内容权限；不会自动发布 Release 或修改官网。

`scripts/test-windows-ci.ps1` 用 PowerShell 7 执行以下检查：

1. 确认 Go 主机架构与测试架构一致，避免把交叉编译当作运行验证。
2. 执行桌面 / internal 测试、vet，以及独立发布工具模块的测试和 vet。
3. 调用现有 `package-windows.ps1`，生成并验证 ZIP 和资源。
4. 比对 ZIP 的 SHA-256，解压后用 Windows 文件版本 API 检查产品和文件版本。
5. 对 ZIP 中的实际 EXE 执行 `TestPackagedApplicationSmoke`：启动、自动退出，再次启动、退出；每次确认到达原生主窗口启动阶段、资料标识正确、无存储错误。
6. 从 SQLite 直接读取状态，验证中文、备注、重要程度、固定状态等字段恢复；由实际应用启动清理过期临时任务并保存删除标记；退出后资料锁可重新获得。
7. 编译按用户安装器；在独立临时主机验证安装、运行中拦截、合成低版本升级、拒绝降级、同版本覆盖、卸载保留资料及重装恢复。具体夹具与验证边界见 [安装器记录](WINDOWS_INSTALLER.md)。
8. build 36 起，安装器测试之前还运行 ZIP 内 EXE 的 Windows 侧栏原生输入检查；窗口区域、焦点与拖动的覆盖及限制见 [输入适配记录](WINDOWS_INPUT.md)。

测试只创建独立合成资料。`-quit-after` 控制正常退出，单次进程另有 45 秒超时。默认测试跳过可执行文件冒烟；显式设置 `SIDELET_SMOKE_EXE` 才运行。`SIDELET_SMOKE_OUTPUT` 可指定诊断父目录，测试始终在其下创建随机新目录，不复用现有资料。

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

CI 检查的是托管 Windows 环境中的自动测试，不是完整人工验收。鼠标跨窗、多屏、输入法候选、通知横幅 / 声音 / 点击仍需单独测试。托管 Windows runner 以管理员身份运行，不能代表普通用户权限下已经通过。清单见 [Windows 打包记录](WINDOWS_PACKAGE.md)。

官方依据：

- <https://docs.github.com/en/actions/reference/runners/github-hosted-runners>
- <https://github.com/actions/setup-go>
- <https://github.com/actions/upload-artifact>
