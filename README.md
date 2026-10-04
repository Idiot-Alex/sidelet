# Sidelet

常驻桌面的轻量 Todo 工具。核心体验：**Glance → Hover → Act**。

当前阶段：**macOS 0.1.0 本地预览版 · 已提供 Apple Silicon DMG 安装包**。P0 原生验收仍保留待验收状态。

当前安装包为构建 5，默认显示 Dock 图标，可在“设置 → 外观”关闭；关闭主窗口后仍在后台运行，点击 Dock 恢复窗口。详见 [Dock 行为记录](docs/phase-1-macos-dock.md)。任务页、设置页和浮动卡片提供“精致 Mac / 温暖纸色 / 深色石墨”三套可保存主题。详见 [UI 与主题记录](docs/phase-1-ui-themes.md)。侧栏继续保留细窄任务标记、空白透明和悬停展开，原修复见 [透明背景记录](docs/phase-1-macos-transparency.md)。

产品规格见 [Revision 4](docs/sidelet-v1-product-spec-revision-4.md)，Windows 原型验收见 [Phase 0 验证记录](docs/phase-0-spike.md)，Mac 进展见 [macOS 原型记录](docs/phase-0-macos.md)，持久化实现与本机验证见 [SQLite 开发记录](docs/phase-1-sqlite.md)。根据当前开发机器，先完成 macOS / Apple Silicon 版本。

## 当前功能

- 我的任务窗口支持新增、编辑、删除、完成 / 恢复、截止时间、重要程度和临时任务。任务列表位于主区域，新增 / 编辑面板位于右侧；截止时间与低频选项可展开。
- macOS 默认显示 Dock 图标；“设置 → 外观 → 在 Dock 中显示”可即时切换并保存，旧设置升级默认显示。隐藏图标后仍保留菜单栏入口。
- “设置 → 外观”切换三套主题，自动保存并同步到任务窗口、桌面标签、快速卡片和 macOS 主窗口标题栏；切换时保留任务草稿。
- SQLite 保存任务、手动固定关系、Stack 位置、密度及任务顺序；重启恢复。新任务默认不在桌面显示，需明确固定。
- 完成后保留 5 秒 Undo；支持基础 Snooze 和快速编辑。临时任务在撤销期结束后移除，重启清理已完成临时任务。
- 保存失败保留草稿，确认写入成功后才退出编辑。
- “整理桌面”模式提供拖动排序和 Option + ↑ / ↓ 调整；超过八条时可在“全部任务”中整理。顺序自动保存，隐藏 / 已完成任务保留原位，Esc 退出后恢复锁定。
- 整理模式中的“移动整组”手柄支持当前屏幕内上下移动和左右换边；松手保存，Esc 取消。也可用方向键预览、Enter 保存。位置预览不写库，保存失败恢复原位。验收范围见 [整组移动记录](docs/phase-1-stack-move.md)。
- 到期前 30 分钟显示“即将到期”，到点显示“已逾期”；明确勾选“提醒我”才发送 macOS 系统通知。支持权限提示、Snooze 延后、重启补发与去重；Sidelet 需在后台运行。详见 [提醒开发记录](docs/phase-1-reminders.md)。
- 一个 Stack 共用一个 WebView。`-spike` 使用 8 条可丢弃内存任务，`-spike -two-stacks` 用于两个 Stack 的原生测试。
- macOS 使用非激活 NSPanel 与 WKWebView；Stack 的 DOM 命中矩形对应独立原生接收面板，共享一个 WebView，任务间隙保持穿透。支持菜单栏入口、全局快捷键、Retina 和工作区布局。
- Windows 平台层保留动态 Window Region、NoActivate、TopMost 和全局键盘入口。
- 两个平台的透明间隙与焦点行为需要在不同下层应用上实测。
- WorkArea 边界计算、动态 `+N` 聚合入口、Quick Card 内容滚动。
- Stack 与 Quick Card 共享悬停状态，跨透明间隙时保留展开；离开两者 500ms 后关闭，编辑与键盘操作期间保持打开。
- 小工作区内的键盘选中任务会临时进入直接显示区，不改变原始排序；Esc 与安静模式退出键盘操作。
- 浏览器交互实验室用于验证状态与布局；Mac `.app` 与 Windows 可执行文件用于原生验收。

依赖锁定：Go `1.27.1`、Wails / 前端 Runtime `v3.0.0-beta.27`、Svelte `5.57.1`。前端包版本与完整依赖树见 `frontend/package-lock.json`。开发机器使用 Node `24.11.0`。

Wails 使用 [v3 beta.27](https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.27) 的多窗口 API。该版本仍处于预发布阶段，窗口方案待 Spike 结果确认。

## 安装本地预览版

打开 `build/releases/Sidelet-0.1.0-local-arm64.dmg`，将 **Sidelet.app** 拖入 **Applications**，再从“应用程序”启动。安装后使用不需要 Go、Node 或 Xcode。首次从空库启动会打开任务窗口；已有偏好选择隐藏主窗口时，点击菜单栏图标进入“我的任务”。

本机安装包采用临时签名，尚未做 Developer ID 签名和 Apple 公证。此包面向本地验收，不作为公开分发包。安装 / 升级说明、校验与限制见 [打包记录](docs/phase-1-macos-package.md)，DMG 内也附有中文说明。原应用标识和资料目录保持不变，已有任务与设置继续使用。

重新制作：

```sh
npm run package:macos
```

输出 DMG 与同名 `.sha256` 校验文件。构建元数据来自 `packaging/macos/app.json`，图标源文件为 `assets/sidelet-icon.svg`。从旧开发目录改为安装使用时，先关闭旧位置的登录启动并退出旧版本，再从 Applications 中按需开启登录启动。

## 在 M3 Mac 上运行

需要 macOS 13 或更新版本、Xcode Command Line Tools、Go 和 Node。当前已构建 Apple Silicon 原生版本：

```sh
npm --prefix frontend ci
npm run build:macos
open "build/bin/Sidelet.app" --args -main
```

`-main` 打开“我的任务”，可以创建任务并设置桌面边缘、中心位置和密度。首次空库且尚无设置时自动打开主窗口；之后遵循设置页的“启动时显示主窗口”。关闭该选项时只显示固定的桌面任务与菜单栏图标。菜单栏可以打开任务窗口、切换安静模式或退出。关闭主窗口只隐藏它。

数据库默认位于 `~/Library/Application Support/Sidelet/sidelet.sqlite3`，所有任务操作自动保存。应用级偏好单独存入同目录的 `settings.json`；可从主窗口或菜单栏打开“设置”，配置外观主题、登录启动、新任务组默认值及通知权限入口。`-data-dir` 可指定独立资料目录，用于测试时不影响日常任务：

```sh
open -n "build/bin/Sidelet.app" --args -main \
  -data-dir "$PWD/build/results/my-test-data"
```

同一资料目录只允许一个产品实例；重复启动会显示已有实例的任务窗口。

Mac 全局键盘入口是 **Control + Option + T**，使用原生 Carbon 独占注册；占用冲突会在界面与日志中显示。`-spike` 验证面板的“进入键盘操作”会隐藏面板并激活真实 Stack，`Esc` 退出后尝试恢复之前的前台应用或面板。普通 Hover 与查看使用非激活窗口；显式键盘操作和编辑才申请焦点。

需要诊断日志时，在终端直接运行：

```sh
mkdir -p build/results
"build/bin/Sidelet.app/Contents/MacOS/sidelet" \
  -main -log-file "$PWD/build/results/macos-spike.log"
```

双 Stack 测试使用 `-spike -two-stacks`，定位原生命中可加 `-trace-pointer`。`-spike` 不打开数据库，退出后不保存测试修改，不能与 `-data-dir` 同时使用。构建产物采用本地临时签名，尚未做发布公证。29 项原生回归与 7 项接收面板 → WKWebView 检查通过；快捷键注册 / 冲突 / 退出释放、真实全局按键到达 Carbon 回调，以及测试入口下 Esc 恢复外部应用和输入焦点已有实测记录。真实快捷键后的完整编辑 / Esc 流程、鼠标穿透和多屏仍需验收，详见 [macOS 原型记录](docs/phase-0-macos.md)。

## 本地交互预览

```sh
npm --prefix frontend ci
npm run dev
```

打开 <http://127.0.0.1:5173/>。在左侧调整位置、工作区高度和标签高度；悬停标签、打开 Quick Card，或点击“进入键盘操作”。浏览器中的 `Ctrl+Alt+T` 仅在当前页面内生效。

```sh
npm run check
npm test
npm run build:frontend
bash scripts/go.sh test -race ./internal/storage ./internal/spike ./cmd/spike ./internal/diagnostics
# macOS 图形登录会话内运行；创建临时测试窗口，结束后自动关闭
npm run test:macos
```

`scripts/go.sh` 优先使用 PATH 中的 Go，找不到时使用项目本地 `.tools/go/bin/go`。模块和构建缓存放在 `.cache/`；工具、缓存、依赖和构建产物均不纳入版本控制。

## macOS 性能采样

省略 `-main` 与 `-trace-pointer` 启动应用，从启动日志读取 `pid=`。在 macOS 图形登录会话中运行：

```sh
npm run profile:macos -- --pid 12345 --duration 600 --interval 5
```

将 `12345` 替换为本次 Sidelet PID。脚本默认预热 30 秒，随后采样 10 分钟，输出 CSV、环境记录、逐进程 `.processes.jsonl` 和统计 JSON 到 `build/results/`。CPU 同时报告单核占用与按逻辑核数归一化的百分比；RSS 与 physical footprint 分开记录。WebKit 归属依据与主进程相同的两种 coalition 标识，避免漏掉父进程为 `launchd` 的辅助进程。

coalition 查询使用 Apple XNU 的私有诊断 ABI，仅用于本地采样，不进入应用；若系统接口不兼容，脚本报错，不猜测归属。建议通过 `open` 启动独立应用实例后采样，避免终端启动的多个应用共享归属。进程退出或 PID 被复用时，采样标记为不完整。

原型数据见 [macOS 性能记录](docs/phase-0-macos-performance.md)，0.1.0 安装版的十分钟空闲、重复操作和恢复观察见 [安装版稳定性记录](docs/phase-1-macos-stability.md)。后者可用 `python3 scripts/analyze-macos-stability.py --dir build/results/stability-native-20261005` 复核，分析工具检查为 `python3 scripts/test-macos-stability-analysis.py`。

卡片内存专项测试先记录至少 30 秒基线，通过真实界面完成 100 次开关，再观察关闭后 600 秒。`--finish-file` 可指定一个尚不存在的路径，在观察结束后创建该文件，让采样正常结束；`--duration` 仍作为最长采样时间。逐进程记录可由 `scripts/analyze-macos-card-memory.py` 与 UI 循环记录、阶段时间一起分析，输出各类进程的 RSS / footprint 变化及时间序列 CSV。统计程序要求完整的 100 次成功记录和十分钟恢复数据，离线检查运行 `python3 scripts/test-macos-memory-analysis.py`。

对象定位可显式开启本地快照：

```sh
open -n "build/bin/Sidelet.app" --args -spike -main \
  -memory-dir "$PWD/build/results/memory-local" \
  -log-file "$PWD/build/results/memory-local.log"
python3 scripts/request-macos-memory.py --dir build/results/memory-local --label warm
# 通过界面执行一批操作，再用不同 label 取快照
python3 scripts/request-macos-memory.py --dir build/results/memory-local --label batch
bash scripts/go.sh tool pprof -top -sample_index=inuse_space \
  -base build/results/memory-local/warm/heap.pprof \
  build/results/memory-local/batch/heap.pprof
```

每个阶段保存 Go 的 heap / allocs / goroutine profile、GC 后 MemStats、原生弱引用对象计数及所有前端视图的组件 / 订阅 / DOM 计数。请求工具会等待视图回复完整；重复 label 被拒绝，启动时不能有遗留的 `request.txt`。未传 `-memory-dir` 时不开启请求轮询、额外桥接订阅、弱引用注册表或主动 GC。开启后每秒检查本地请求，并将 Go 内存采样率设为 64 KiB；取快照会强制 Go GC，因此该运行不作为正常空闲性能基线，也不代表 WebKit 的完整 JavaScript 堆。

对采样器确认属于该实例的 WebContent PID，可另用系统 `heap PID` 保存 malloc 对象分类，运行 `scripts/analyze-macos-heap.py --before 前一份.log --after 后一份.log --output 差异.json` 比较。分析器检查运行 `python3 scripts/test-macos-heap-analysis.py`；系统工具的暂停和采样也会干扰性能，须和无诊断基线分开。

独立接收面板 → WKWebView 验证窗口使用 `bash scripts/build-macos-routing-fixture.sh` 构建；界面点击由 Computer Use 或真实鼠标执行，程序本身不投递合成输入。下层滚动、焦点与全屏测试窗口使用 `bash scripts/build-macos-interaction-fixture.sh` 构建。

## macOS 快捷键与外部焦点复测

先关闭已有原型实例，再启动带焦点日志的测试版本：

```sh
open -n "build/bin/Sidelet.app" --args -spike -main -interaction-test \
  -log-file "$PWD/build/results/macos-focus.log"
bash scripts/build-macos-interaction-fixture.sh
# 将 12345 替换为本次 macos-focus.log 首行的 Sidelet PID
open -n build/results/MacInteractionFixture.app --args \
  "$PWD/build/results/macos-focus-fixture.log" 12345
```

在夹具输入框输入文字，点击 **Test-only Sidelet keyboard entry**，然后测试 ↓、Enter、E、取消编辑和连续两次 Esc。第一次 Esc 留在 KeyboardActive，第二次恢复外部应用；无需重新点击输入框即可输入。另测试编辑期间点击 **Activate test app**，Sidelet 应释放输入状态，不主动恢复或抢回焦点。这个按钮使用显式开启的本地测试通知，共用产品控制器，日志标为 `source=fixture-request`；默认运行不开启接收入口，不投递合成按键，也不证明系统全局快捷键交付。真实按下 Control + Option + T 时日志必须出现 `source=global-shortcut`。

仅检查注册占用可运行：

```sh
bash scripts/build-macos-shortcut-probe.sh
build/results/macos-shortcut-probe free      # 原型未运行：注册并立即释放
build/results/macos-shortcut-probe occupied  # 原型运行：预期 OSStatus=-9878
# 原型退出后先占用 20 秒，再在另一终端启动原型，检查冲突提示
build/results/macos-shortcut-probe hold 20
```

探针只注册 / 释放快捷键，不发送按键。冲突解除后需要重启原型，当前没有自动重试。测试期间先只读日志核对系统前台 PID、窗口和输入焦点，再进行输入或点击；终端命令或再次激活夹具可能掩盖错误的恢复结果。

### 准备最后一轮真实键鼠验收

退出已有原型和夹具，构建后运行：

```sh
npm run build:macos
bash scripts/build-macos-interaction-fixture.sh
python3 scripts/start-macos-acceptance.py
```

启动器创建独立的 `build/results/macos-manual-...` 会话目录，记录二进制哈希和两个 PID，等待快捷键与视图就绪，再打开下层测试窗口。同时保存 `sidelet.stdout.log` / `sidelet.stderr.log`，保留运行时错误和线程转储输出。启动的 Sidelet 使用 `-spike -trace-focus -trace-pointer`，不会打开用户数据库，不启用测试键盘通知入口；夹具隐藏测试按钮，显示中文步骤。已有实例或输出目录会被拒绝，避免混用日志。失败时保存启动证据，不自动结束其他进程。

按照窗口说明操作真实键盘和鼠标，恢复后不点击输入框、不切换终端，直接输入 `after-escape`；Hover 后输入 `after-hover`。测完再运行：

```sh
# 将目录替换为启动器输出的本次会话路径
python3 scripts/check-macos-acceptance.py build/results/macos-manual-... \
  --manual-input-confirmed
python3 scripts/test-macos-acceptance.py
```

`--manual-input-confirmed` 是操作者对真实输入来源的声明，只有确实使用真实键鼠才填写；不从 trusted 事件或注册成功推断硬件输入。检查器只认可 Carbon 回调的 `source=global-shortcut`，使用系统前台与 first responder 核对编辑、取消、Esc 恢复和无重新点击的继续输入。间隙 / 透明区点击与滚轮按当时安装的原生矩形分类，滚轮还须改变下层滚动位置；点击其他位置不算 Overlay 穿透证据。报告写入本次 `report.json`，退出码为通过 0、失败 1、待验收 2。报告只覆盖本轮输入场景，不自动升级 P0；Passive 卡片的完整操作、不同下层应用、普通 Space、多屏 / 拔插 / Dock 仍须另外记录。结束时关闭夹具，并从 Sidelet 菜单栏退出。

## 构建 Windows 原型

Windows 需要 Go `1.27.1` 或更新版本、Node `24.11.0` 或兼容版本，以及已安装的 WebView2 Runtime。

```powershell
./scripts/build-windows.ps1 -Architecture amd64
```

在 macOS 上交叉编译：

```sh
bash scripts/build-windows.sh amd64
bash scripts/build-windows.sh arm64
```

产物：`build/bin/sidelet-spike-amd64.exe` 或 `build/bin/sidelet-spike-arm64.exe`。Go 构建会嵌入 `frontend/dist`，无需另外拷贝前端文件。

## Windows 运行与采样

在 Windows PowerShell 中启动，并把日志保留在本地：

```powershell
New-Item -ItemType Directory -Force build/results | Out-Null
$sideletRun = Start-Process ./build/bin/sidelet-spike-amd64.exe `
    -ArgumentList '-main' -PassThru `
    -RedirectStandardError ./build/results/spike.log

./scripts/profile-windows.ps1 -ProcessId $sideletRun.Id
```

正常空闲场景省略 `-main`；双 Stack 场景使用 `-spike -two-stacks`。主窗口可从托盘打开或隐藏；关闭窗口只隐藏，托盘“退出 Sidelet”结束程序。Windows 持久化代码已通过交叉编译，原生运行尚未验收。

`Ctrl+Alt+T` 在 Windows 注册为全局键盘入口。普通 Hover 和 Quick Card 查看保持 Passive；编辑或显式键盘模式才获取焦点。全局快捷键注册失败会写入日志。

原型运行期间，Hover 会记录约 200ms 的帧回调采样与首次回调延迟。空闲时不持续运行帧采样或轮询任务数据。

## 代码入口

```text
cmd/spike/                  Mac / Windows 启动、串行控制器与保存结果发布
internal/platform/          原生窗口、命中区域、显示器与焦点边界
internal/spike/             可丢弃的内存假数据
internal/todo/              任务与桌面布局共享模型
internal/storage/           SQLite、迁移与事务式任务操作
frontend/src/components/    TodoManager / EdgeStack / QuickCard
frontend/src/lib/           几何计算、预览状态、Wails 消息桥
scripts/                   Mac / Windows 构建、Mac 原生回归与 Mac / Windows 性能采样
```

本轮按用户指示先推进 SQLite 与基础任务管理，P0 未验收项目继续单独记录，Revision 4 原文不变。桌面任务排序已接入，详见 [排序开发记录](docs/phase-1-reorder.md)。整组移动已接入，见 [整组移动记录](docs/phase-1-stack-move.md)。到期状态与 macOS 系统提醒已接入，见 [提醒开发记录](docs/phase-1-reminders.md)。统一设置已接入，见 [设置开发记录](docs/phase-1-settings.md)。完整流程回归见 [回归记录](docs/phase-1-regression.md)，可运行 `npm run test:regression` 重复自动化检查；直接鼠标拖动、多屏和原生交互验收继续保留。
