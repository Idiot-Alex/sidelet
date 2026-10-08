# MyGo 原生侧栏实验

2026-10-07。**完成独立原型及 macOS 原生输入适配，暂不替换 Wails。** 首轮发现的透明空白拦截点击和侧栏抢焦点，已通过独立的非激活输入面板处理。后台悬停和打开卡片已有真实事件证据；Computer Use 验证了拖动、拼音输入、保存及连续编辑取消，并修复了事件坐标和重复编辑焦点问题。跨应用焦点恢复、正常输入面板的持续拖动及多屏仍未完整验收。

## 范围与运行

固定依赖 `github.com/egoist/mygo v0.2.16`，独立 `go.mod` / `go.sum`，Go 1.27.1，`CGO_ENABLED=0`。通过本地模块引用复用 `internal/todo` 的时间计算、`internal/reminder.Sync` 的提醒协调和 `internal/spike` 的卡片会话，不引入 Wails、SQLite 或前端运行时。`-native-main` 默认使用独立 JSON 资料，首次为空列表，已提交任务、顺序、主题、布局和提醒回执可重启恢复；历史侧栏 / 混合实验仍使用三条合成内存任务。

```bash
# 项目根目录；构建会检查正式 UI 参照并执行实验的 74 项顶层测试及 go vet。
bash experiments/mygo-sidebar/build-macos.sh
open build/bin/mygo-lab/SideletMyGoLab.app
```

默认左侧显示三条细窄任务标记。悬停展开，点击展开的内容打开任务卡片。拖动标记移动整组，释放后吸附到左 / 右边缘。原生卡片可编辑、保存、取消与完成。通过自己的菜单退出实验应用。

另已实现 `-hybrid`：同一 MyGo 宿主内使用 Web 主窗口和原生侧栏共享任务，支持双向更新、完成和关闭重开，已验证跨应用焦点恢复。具体范围、运行方式和历史内存采样见 [混合原型记录](HYBRID.md)。`-native-main` 使用全原生主窗口，不创建 WebView；现在按正式 Sidelet 的完整 UI 逐项对齐，包括任务页、设置、三主题和浮动卡片。**整体 UI 一致性尚未验收完成**，已实现内容、禁用能力与截图证据见 [UI 对齐记录](UI_PARITY.md)。[原生主窗口记录](NATIVE_MAIN.md)中的内存比较属于较早的简化界面。默认不传这些选项仍为原生侧栏实验。

应用标识 `io.sidelet.mygo-lab`，显示名 `Sidelet MyGo Lab`。不会覆盖 `/Applications/Sidelet.app`。当前 macOS 包只是本地 ad-hoc 签名实验，未公证、未发布。`-native-main` 支持主窗口 / 侧栏整理排序和 +N 溢出卡片，满屏新增、撤销和恢复不会被拒绝；支持截止时间、可选 macOS 提醒及完成 5 秒后清理的临时任务。安静模式隐藏桌面标签 / 卡片并撤销鼠标区域，系统提醒继续运行；恢复和重启重新显示。设置页可保存启动窗口偏好、默认值与 macOS Dock 显示，Dock 隐藏时保留独立菜单栏入口。默认值不改变现有任务组；当前仅单组，多组创建尚未迁移。登录启动与导出仍禁用。持久资料不读取正式 SQLite 数据，目录、失败处理和提醒恢复边界见 [持久化记录](PERSISTENCE.md)。通知权限属于独立实验标识，首次开启需要授权；不自动请求，也不更改正式 Sidelet 的通知设置。Windows 目前仅通过编译检查，通知适配显示为不支持。

```bash
# 全原生主窗口，默认保存到 ~/Library/Application Support/SideletMyGoLab
open -n build/bin/mygo-lab/SideletMyGoLab.app --args -native-main
# 使用独立测试资料；再次运行同一命令可恢复
open -n build/bin/mygo-lab/SideletMyGoLab.app --args \
  -native-main -data-dir "$PWD/build/results/mygo-manual-profile"
# 保留三条合成任务的可丢弃内存模式
open -n build/bin/mygo-lab/SideletMyGoLab.app --args -native-main -memory
```

`-data-dir` 与 `-memory` 仅用于 `-native-main`，不能同时传入。同一资料目录只允许一个实验进程写入。`-main` 强制打开任务窗口，`-main-hidden` 强制隐藏；两者互斥。正常启动遵循保存的“启动时显示主窗口”，新资料首次仍显示窗口。

自有背景窗口用于检查点击穿透：

```bash
mkdir -p build/results/mygo-lab-manual
open -n build/bin/mygo-lab/SideletMyGoLab.app --args \
  -probe \
  -output "$PWD/build/results/mygo-lab-manual" \
  -log-file "$PWD/build/results/mygo-lab-manual/app.log"
```

`-probe` 打开实验自己的计数器窗口，和正式任务数据无关。收起任务卡片后，在下层按钮上比较侧栏范围内的透明区与范围外的点击。`-card` 可用于启动即显示第一条合成任务卡片；卡片覆盖按钮时不用于侧栏空白穿透测试。

`-output` 把宿主数据指向指定目录内的 `app-data/`；持久模式未显式传 `-data-dir` 时，任务资料放在该目录的 `profile/`。仅在此选项启用时监听 SIGUSR1，收到后保存自己的原生内容 PNG、Go 内存统计和焦点状态；不强制 GC，不注入系统输入。`-quit-after 2m` 可限制一次实验的运行时间。显式诊断会包含任务内容，普通持久模式不输出完整任务快照。

## 首轮验证（输入适配前）

| 项目 | 结果与证据边界 |
| --- | --- |
| Go 绘制界面 | 实际 macOS 应用启动、显示中文任务；窗口 `Content` 使用 `ui.View`，没有 HTML / JS 前端 |
| 透明绘制 | 真实原生内容截图 624×332；空白、间隙和角落 alpha=0，标记 alpha=255。Headless 测试也覆盖悬浮后仅当前行绘制背景 |
| 悬停与进入预览 | Headless 指针测试覆盖从把手移入预览、离开、切换任务及右侧模式；实机点击把手后出现预览并可打开卡片。未把它升级为后台纯悬停的实机验收 |
| 编辑 / 保存 / 取消 | 实机 AX 写入中文、保存、取消保留原值；卡片位置始终为 x=20 / y=419。添加 AppKit first responder 适配后，字母按键与 Enter、Esc 取消有效 |
| 中文输入法 | Headless composition / commit 测试通过；CUA `type_text` 直接输入中文未生效。未验证真实输入法选词、候选窗口和连续中文输入，不能宣布实机 IME 通过 |
| 完成任务 | Headless 测试通过，状态只影响合成任务 |
| 拖动 | 实机日志记录移动后的位置；模型测试覆盖原点固定的位移、边界限制、左右吸附与取消恢复。持续跨侧拖动、跨屏及失焦取消仍未实机验收 |
| 空白点击穿透 | **失败**。同一自有下层按钮，在侧栏透明区域 x=200 / y=115 点击后计数仍为 0，侧栏成为 key window；在侧栏范围外 x=400 / y=115 点击后计数变为 1。保存于 `native-ui-final/blank-blocked-ax.txt`、截图和日志 |
| 被动焦点 | **失败**。`ShowInactive` 显示时窗口未聚焦，但点击侧栏会成为 key window；它不能代替非激活面板 |
| Windows | 最终源码通过 amd64 / arm64、无 cgo 交叉编译；没有 Windows 实机运行结论 |

macOS 编辑适配通过 `NativeHandle` 对本实验窗口调用公开 AppKit 的 `makeFirstResponder:`。没有修改 MyGo 依赖源码、替换 Objective-C 类或注入键鼠事件。中文 AX 保存和 headless IME 测试不能代替真实输入法验证。

审阅固定版本源码还发现：`internal/darwin/surface.go` 的原生视图 tracking area 使用 `NSTrackingActiveInActiveApp`。因此后台侧栏悬停还需要额外验证 / 适配。透明背景、整个窗口忽略鼠标和逐区域鼠标穿透是不同能力：直接 `SetIgnoreMouseEvents(true)` 会连任务标记一起失去鼠标输入。

## 内存记录

Apple M3 Max / macOS 26.6.2，主显示器 1728×1117、2×。同一 `scripts/profile-macos.py` 读取 RSS 和 `ri_phys_footprint`，CPU 100% 表示占满一个逻辑核心。Sidelet 汇总匹配 resource / jetsam coalition 的 WebKit 进程；采样不强制 GC、不退出 WebKit 来制造下降。

首轮参考：两者均三条相同中文合成任务、一个侧栏、未打开主窗口 / 卡片，20 秒、每 5 秒一次，共 5 点，取中位数。

| 首轮后台参考 | RSS | footprint | WebContent |
| --- | ---: | ---: | ---: |
| MyGo 原型（初版） | 99.73 MiB | 24.97 MiB | 0 |
| Sidelet 构建 25（完整应用、隔离数据库） | 189.58 MiB | 65.39 MiB | 1 |

这不是相同完整功能的框架基准。Sidelet 还包含存储、提醒、快捷键与原生输入路由等能力；MyGo 只有实验界面。不能据此承诺迁移后整机内存减少某个百分比，也不能套用到打开主窗口后的原有 469 MiB / 231 MiB 数字。

证据总目录 `build/results/mygo-sidebar-20261007/`，被 Git 忽略。各目录含应用日志、CSV、逐进程 JSONL、机器 / 二进制元数据和摘要：

- `idle-1/` 的 `mygo-native-idle-1` 与 `wails-idle/` 是上述首轮参考。初轮终端启动的应用会继承终端 coalition；当时 MyGo 静置没有 WebKit，Sidelet 独立采样期间未有交互。此结果仅作参考。
- `idle-1/` 的 `mygo-native-card-used` **弃用**：两份应用通过同一终端启动、共享 coalition，把对照的 WebKit 计入了原型。不能用于原生卡片内存结论。
- `native-final-idle/` / `wails-final-idle/` 改用 Launch Services 启动，已确认 coalition 分别为 28985/28986 与 28993/28994，真正分离。该轮采样期间发生拖动与悬停，不能当作冷启动静置基准。MyGo footprint 一度约 235 MiB，随后回落到约 29 MiB；Sidelet 当轮已创建两个 WebContent，也不和一个 WebContent 的冷启动状态混比。
- `native-ui-final/` 保存最终界面的 alpha、焦点、中文 AX 保存 / 键盘取消及点击穿透失败证据。
- `native-final-cold-2/` 是最终源码版本、正常 macOS 启动方式的一次短时后台复测，具体值以该目录摘要为准；如果日志存在采样期间交互，不视为静置结果。

首轮最终版本的后台复测日志只有 `ready`，采样期间没有悬浮 / 拖动 / 打开卡片事件：RSS **95.16 MiB**、footprint **22.78 MiB**，单个进程、0 个 WebKit 子进程，CPU 一核平均 **0.0050%**。20 秒内内存读数相同、进程集合稳定。它确认了小型原生侧栏的静置内存表现，仍不是完整 Sidelet 的迁移收益。

这轮没有跑长时间压力测试。GPU 临时资源可令 footprint 高于 RSS，也可在交互结束后明显回落；应分别观察冷启动、拖动峰值、编辑与恢复，而不是用最低的一次读数代替全部场景。短时 CPU 采样也不足以得出更省电的结论。

## macOS 输入适配（第二轮）

渲染窗口保持忽略鼠标；只有可见标记、展开行和阅读卡片才有独立 `NSPanel` 输入区域。面板使用 [`nonactivatingPanel`](https://developer.apple.com/documentation/appkit/nswindow/stylemask-swift.struct/nonactivatingpanel)，不能成为 key / main window，位于对应渲染窗口下方；空白处没有输入面板，圆角也保持透明。

输入视图使用 [`NSTrackingActiveAlways`](https://developer.apple.com/documentation/appkit/nstrackingarea/options/1530540-activealways)。收到真实 AppKit 鼠标事件后，只转换坐标并交给 MyGo 原有内容视图，保留按钮、布局、拖动捕获和文本输入。没有轮询全局鼠标、修改依赖源码或替换 MyGo 的 Objective-C 类。

点击编辑时才让卡片接收原生输入和键盘焦点。保存、取消或关闭后恢复被动模式；原先的普通窗口恢复焦点。用户在编辑期间主动切换到其他应用时，不会强行把焦点拉回来。标记完成后隐藏对应面板，退出时关闭和释放面板、清空注册表。Windows 尚未实现对应输入适配。

新增几何测试逐像素对照原生 UI 的实际绘制，覆盖左右侧、展开行、拖动隐藏预览和完成任务后的输入区域。原有六项测试继续通过。

另提供自有 AppKit 窗口诊断。先退出正在运行的实验，再从项目根目录执行：

```bash
mkdir -p build/results/mygo-native-check
open -n build/bin/mygo-lab/SideletMyGoLab.app --args \
  -probe -native-check \
  -output "$PWD/build/results/mygo-native-check" \
  -log-file "$PWD/build/results/mygo-native-check/app.log"
```

诊断约三秒后自动退出，在输出目录写入 `native-check.json`。当前十个阶段均通过：静置、展开预览、阅读卡片、编辑、取消恢复、重复编辑、移动、取消拖动、完成任务和清理。它读取 AppKit 的真实 key window、非激活面板样式、后台 tracking 选项及 `windowNumberAtPoint:belowWindowWithWindowNumber:` 的预测点击目标；也检查透明空白和圆角不会命中输入面板、卡片取消不会改变位置。

**此诊断直接切换实验模型和自己的窗口，不创建或投递系统键鼠事件，不能代替真实鼠标悬停 / 拖动或中文输入法验收。** CUA 在下层窗口上的空白点击使计数增加，但本轮 CUA 标记点击 / 拖动没有触发输入面板事件，所以不将其宣称为硬件路径通过。当前 `native-check.json` 也明确记录 `hardwareHoverVerified: false`。

第二轮证据目录：`build/results/mygo-sidebar-20261007/native-adapter/`。`check-final/native-check.json` 是最终版本的九阶段报告；旧的失败记录仍保留在上一轮目录中。生产 Sidelet 的源码、依赖、安装和任务数据库均未被本实验替换。

加入输入面板后，最终二进制再用 Launch Services 独立启动，静置 20 秒 / 每 5 秒采样：RSS **95.92 MiB**、footprint **23.38 MiB**，单进程、0 个 WebKit，CPU 一核平均 **0.0075%**。日志只有 `ready`，五个采样点的内存相同、进程集合稳定；计入这次空白侧栏的三个输入面板，没有打开卡片或诊断背景窗口。相较上一轮短测仅小幅变动，不能把单次差值当作框架或完整业务的固定开销。`native-adapter/idle/` 保存数据，`native-adapter/build.json` 保存对应二进制哈希和验证摘要。

## Computer Use 与真实事件复测（第三轮）

`-trace-input` 仅观察本实验收到的鼠标事件，原样返回，不监控其他应用的键盘，不生成系统输入。正常输入面板模式的 `real-input/app.log` 记录了实际后台悬停、从把手进入展开内容、打开和关闭卡片：事件期间前台应用 PID 保持不变，渲染窗口未成为 key window。这部分有正常系统输入路由的证据，但该段拖动只是小幅点击动作，不能代表完整拖动验收。

Computer Use 的坐标操作直接向应用窗口投递事件，不移动系统鼠标，也无法定位默认模式中隐藏于辅助功能树的非激活输入面板。提供 `-direct-input-fixture -trace-input -output <目录>` 供工具测试渲染窗口中的相同控件；该选项关闭实验输入面板，禁止与 `-native-check`、`-probe-only` 混用，日志标记 `systemOverlayRoutingVerified: false`。它只是测试入口，不是默认运行方式。

这轮发现并修复了三个具体问题：

- 拖动改用收到的事件坐标，不再采样可能滞后的系统鼠标位置。
- 窗口随指针移动时，连续事件的窗口内坐标可能相同，MyGo 会跳过重复的 pointer move。macOS 输入适配现在直接处理每个实际收到的拖动 / 释放坐标，并在结束时交回原有控件的释放流程；没有改动依赖源码。
- 保存后再次编辑，原生 key window 状态和复用文本框的焦点未正确恢复。返回阅读模式时在同一事件循环内隐藏并非激活重显卡片；每次编辑显式聚焦文本框，取消仍保留标题和位置。

| 第三轮检查 | 结果与边界 |
| --- | --- |
| Go 测试 / vet | 十项测试及 vet 通过；增加移动窗口中的事件坐标、相同局部坐标的连续拖动、反复编辑焦点回归 |
| AppKit 诊断 | 十阶段通过；新增抬高卡片后的重复编辑 / 取消 / 再编辑检查。仍属于自有窗口诊断，不是硬件输入 |
| Computer Use 拖动 | 最新构建从 y=411 连续拖到 y=471，完整移动 60 点；吸附右侧 x=1416，再回左侧 x=0，y 保持 471。使用直接投递测试入口 |
| 系统拼音输入 | 用单个按键输入 `nihao` 并空格选词，保存“你好”；再编辑自动聚焦，`ni` 显示组合态，Esc 取消组合后编辑器仍打开；点击取消保留“你好”，位置仍 x=20 / y=419；第三次输入 `shijie` 并保存“世界” |
| 后台窗口继续输入 | 独立合成输入窗口可通过工具继续输入“你好”。工具会向指定应用投递按键，因此此结果不能证明操作系统已恢复该应用的前台焦点；跨应用恢复仍待验证 |
| Windows | 最新源码 amd64 / arm64 交叉编译通过，未做 Windows 运行验收 |

证据在 `build/results/mygo-sidebar-20261007/`：`real-input/app.log` 保存正常模式的真实后台事件；`real-input/check-drag-final/native-check.json` 为最新十阶段报告；`cua-input/final/` 保存连续编辑、拼音组合截图及取消日志；`cua-input/drag-final/` 保存最终拖动修复的日志。修复前的失败证据继续保留。

本轮没有重新采样内存。上文 95.92 MiB / 23.38 MiB 属于第二轮对应哈希的短测，不能当作本轮新二进制的测量结果。生产 Sidelet、任务数据和已安装应用未被替换。

## 下一阶段的判定条件

继续验证正常非激活输入面板中的持续拖动、编辑后跨应用恢复输入，以及中文候选窗口 / 长文本、跨屏 / DPI / Spaces。现有后台事件和工具输入记录只覆盖上述场景。满足这些要求后，才做带相同业务能力的原生侧栏对照。

MyGo 支持同一应用混用 Web 和 Go 绘制窗口，值得继续探索主窗口 Web、常驻侧栏原生的方案。但 MyGo 与 Wails 各自管理应用事件循环，不能把 `ui.View` 当成现有 Wails WebView 的直接替换项；需要独立评估宿主和原生接口的迁移工作。

参考：[MyGo v0.2.16](https://github.com/egoist/mygo/tree/v0.2.16)、[原生窗口文档](https://github.com/egoist/mygo/blob/v0.2.16/docs/ui/windows.md)。
