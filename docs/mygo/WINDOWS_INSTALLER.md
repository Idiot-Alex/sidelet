# Windows 安装器

2026-10-23；当前版本 0.2.0 build 36，安装器从 build 35 引入。基于现有经过验证的 ZIP，新增按当前用户安装的 NSIS 安装包。未做 Authenticode 签名，不自动发布 Release。

## 构建

在 Windows PowerShell 7 中运行：

```powershell
.\scripts\build-windows-installer.ps1 -Arch amd64
.\scripts\build-windows-installer.ps1 -Arch arm64
```

入口先执行现有测试与 ZIP 打包；`-SkipBuild` 复用同版本 ZIP，但仍检查 SHA-256、固定四文件清单、产品和版本信息。使用已固定 SHA-256 的 NSIS 3.13 ZIP，在临时构建目录解压，不安装到构建机器的系统目录。编译失败不替换现有安装包。

产物：`build/releases/Sidelet-0.2.0-build36-windows-<arch>-local-setup.exe` 及 `.sha256`。多尺寸图标来自应用的同一资源。Mac 上通过仓库 Windows CI 构建和执行安装器。

## 用户流程与保护

- 安装向导支持简体中文和英文提示。
- 安装到 `%LOCALAPPDATA%\Programs\Sidelet`，请求普通用户权限，注册当前用户的开始菜单快捷方式及 Windows 应用卸载入口。
- SQLite 继续位于 `%APPDATA%\SideletMyGoLab`；升级、卸载不读取、修改或删除该资料目录。
- 覆盖升级要求原应用已退出；运行中或 EXE 被锁定时返回错误 10，不终止进程。并行安装/卸载返回 11。
- 支持同架构升级及同版本覆盖修复；拒绝降级（14）、跨架构覆盖或改变既有安装目录（13）。更换架构应先卸载，保留的数据可在重新安装后读取。
- 不自动启动应用，不自动添加登录启动或桌面快捷方式。完成后从开始菜单启动。
- 卸载按明确文件清单删除程序及快捷方式、HKCU 注册，不递归删除安装目录；用户自行放入的文件保留。
- 文件或注册表写入失败会明确报错；不提供断电/磁盘故障下的事务回滚，可重新运行同版本安装器修复。安装前的 EXE 占用检查无法完全排除检查后另一进程启动的竞争，后续写入失败仍会报错。

静默安装：`setup.exe /S`。静默卸载使用注册的 `QuietUninstallString`。NSIS 的卸载器通常从临时副本运行；自动验收显式复制后使用 `_?=` 等待实际卸载进程，避免把外层启动器退出误判为卸载完成。

## 验证

`scripts/test-windows-installer.ps1` 只允许在 Windows GitHub Actions 临时主机运行，且检测到原有 Sidelet 安装、快捷方式、注册或资料即拒绝；不允许在用户日常 Windows 机器上直接执行此破坏性生命周期测试。

两架构 CI 执行：

1. 安装合成的低版本安装包，检查当前用户目录、开始菜单目标与 HKCU 卸载注册。
2. 使用前一步打包 EXE 冒烟测试的真实合成 SQLite 资料，启动已安装程序；运行中升级和卸载必须拒绝且不终止程序。
3. 退出后升级到本轮包，检查 EXE 字节确实替换、版本注册更新、SQLite 文件哈希不变。
4. 拒绝旧安装包降级，同版本覆盖修复成功；已安装应用恢复中文任务。
5. 卸载后检查资料哈希、无关文件保留，程序/注册/快捷方式移除；重新安装并启动，验证任务恢复。

低版本安装包是测试夹具：复用本轮程序并附加可区分的 PE 尾部数据、设置较低的安装器版本。它验证安装器替换与版本判断，不冒充历史发布包或旧数据库迁移验证。

本地 Go 发布工具测试与 vet、桌面 / internal 测试通过。验证源码 `83d0bb2` 的 [Windows CI](https://github.com/Idiot-Alex/sidelet/actions/runs/39642384601) 两架构全部通过：x64 2 分 37 秒，ARM64 3 分 11 秒，包含上面列出的完整安装器生命周期检查。两种安装包与 ZIP 已下载并重新核对 SHA-256；ZIP 清单、CRC、版本、架构和干净源码提交也已复验，产物保存在 `build/releases/`，校验记录在 `build/results/windows-installer-ci-39642384601/verified.json`。

build 36 的 [双架构复验](https://github.com/Idiot-Alex/sidelet/actions/runs/39809334284) 继续通过上述安装器生命周期检查，验证源码 `12865c6`；本轮新增的输入适配见 [记录](WINDOWS_INPUT.md)。

托管 runner 为管理员账户，即便安装器声明 `RequestExecutionLevel user`、只写 HKCU 和用户目录，仍不能替代普通用户、UAC、SmartScreen 和安装向导交互验收。

参考：NSIS 官方命令行和脚本手册：

- https://nsis.sourceforge.io/Docs/Chapter3.html
- https://nsis.sourceforge.io/Docs/Chapter4.html
- https://nsis.sourceforge.io/Docs/AppendixD.html
