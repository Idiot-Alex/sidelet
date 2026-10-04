# Phase 0 · macOS / Apple Silicon

状态：**Development Candidate，部分原生交互通过，P0 尚未通过**。

用户当前使用 M3 Mac，要求先做 macOS 版本。本记录覆盖 Phase 0 的 8 条内存假任务与原生窗口交互，现需使用 `-spike` 显式启动。2026-10-05 按用户指示先推进 [SQLite 与基础任务管理](phase-1-sqlite.md)，日常启动已使用持久化；Revision 4 原文不变，P0 状态未升级。构建和启动命令见 [README](../README.md#在-m3-mac-上运行)。

## 已实现

- Apple Silicon arm64 `.app`，最低构建目标 macOS 13，前端资源嵌入可执行文件，使用系统 WKWebView。
- 菜单栏应用，支持验证面板、安静模式、重置假数据和退出；默认不显示 Dock 图标。
- 1～2 个透明 Stack NSPanel 与复用的 Quick Card。普通查看保持非激活，显式键盘模式或编辑允许取得焦点。
- Control + Option + T 使用原生 Carbon 独占注册；明确绑定 Control / Option 修饰键，冲突在界面与日志中显示。关闭绑定窗口时清理注册与事件处理器。
- Stack 的渲染窗口始终鼠标穿透；DOM 上报的每个命中矩形对应一个非激活原生接收 NSPanel，位于渲染窗口下方。接收面板共用既有 WKWebView，在事件到达前安装好；展开、收起、移动、隐藏与关闭会同步更新或清理，间隙没有接收面板。
- 接收面板将已投递给本应用的鼠标事件交给同一个渲染窗口，保持屏幕坐标、点击次数、修饰键、时间戳与压力。接收面板不允许取得键盘焦点，也不创建额外 WebView。Quick Card 直接使用完整矩形窗口，滚轮交给原 WKWebView。
- 鼠标观察仅更新 Hover 和外部点击状态，不再控制首次点击路由。坐标以实际 WKWebView 为准，忽略不在任何显示器范围内的占位坐标；窗口标题补齐以便原生可访问性识别。
- 显示器采用稳定 UUID；菜单栏与 Dock 占用通过 `NSScreen.visibleFrame` 排除。Mac 布局使用逻辑点，日志 `coordinateScale=1`，Retina 的 `backingScale=2` 单独记录，避免把逻辑点重复放大。
- 屏幕、工作区和前台应用变化使用通知；普通桌面空闲时没有全屏轮询。已经识别的全屏会话每秒检查退出，恢复后停止。鼠标观察只监听鼠标事件，不监听全局按键。
- 键盘模式保存前台应用，退出时尝试恢复。对于外部应用，恢复的是应用而非精确的某一个窗口。
- 全屏几何检测计入刘海屏 safe area；前台和 Space 通知后追加延迟检查，输入入口也检查全屏，防止动画期间的暂态结果或同一 Space 的无边框窗口绕过安静状态。

Mac 不使用 Windows 的 `SetWindowRgn`。旧方案通过全局鼠标通知切换窗口接收状态，存在首次点击先到下层的竞态；[AppKit 全局事件观察](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/MonitoringEvents/MonitoringEvents.html)不能修改或阻止事件投递。新方案由事先安装的原生矩形承担路由；每条可见标签增加一个轻量 NSPanel，一个 Stack 仍只有一个 WebView。独立原生夹具已验证接收面板 → WKWebView 的实际转发路径，真实硬件及不同下层应用的完整验收仍未完成。

窗口数量的取舍需要保留：8 条标签对应 1 个渲染 NSPanel 和最多 8 个接收 NSPanel。接收面板不创建 WebView，但仍是原生窗口；因此它满足单 WebView 的资源约束，尚不能把它等同于规格中“1 个 EdgeStack Window”的字面要求。P0 完成前需评估这一方案。

## 本机验证

| 检查 | 结果 |
|---|---|
| Svelte / TypeScript 与前端生产构建 | 通过，0 errors / 0 warnings |
| 前端几何、假数据与内存组件计数 | 12 项通过 |
| Go 假数据与跨窗口查看会话 | 6 项通过，包含 race 检查 |
| Go 全屏检查生命周期 | 2 项通过，包含 race 检查；普通空闲不装定时器，退出取消，单次检查由控制器重设 |
| 原生回调消息队列 | 3 项通过，包含 race 检查；512 条突发、8 个并发生产者的 4096 条有序事件、唤醒及关闭 |
| macOS 平台代码 vet / 原生链接 | 通过 |
| arm64 Mach-O、Info.plist 与本地签名校验 | 通过 |
| 本机原生启动与初始布局 | 通过：Stack 可见、8 个命中区域安装、键盘焦点未取得 |
| Retina 工作区日志 | 逻辑工作区 1728 × 1007，顶部 33，backingScale=2 |
| 原生 Hover 回调 | 日志观察到展开延迟约 150～151ms、短时帧回调约 61～64fps；不是完整画面性能验收 |
| Windows 平台兼容性 | amd64 / arm64 交叉编译均通过 |
| 原生 Quick Card 标题 / 多行备注保存与重新打开 | 通过，使用实际 WKWebView 编辑字段验证 |
| 原生编辑取消 | 通过，未保存的标题没有覆盖已保存内容 |
| Quick Card 长备注原生滚动 | 22 行备注保存后可滚动至末行；标题与底部操作栏保持可见 |
| 原生任务完成 / 5 秒 Undo | 通过，包括 Stack 的 Space 完成与原生撤销按钮 |
| 原生 ↑ / ↓ 选择与 Enter 打开卡片 | 通过 |
| 原生 KeyboardActive → E 编辑 → Esc 取消 → Esc 退出 | 修复焦点切换后通过；退出恢复到之前的验证面板 |
| 原生 S Snooze / 重置 | 通过，S 后剩余 7 条，重置恢复 8 条 |
| 左侧、offset=1、64px 标签、最后一条任务卡片 | 通过：Quick Card 位于 x=305、y=640、320×390，处于工作区内 |
| 安静模式控制 | 显示恢复入口并禁用键盘入口；完整系统行为仍待验收 |
| 透明区域与标签间隙的坐标点击 | 临时下层原生窗口收到对应点击；保留工具事件路径限制，未将此升级为完整 P0 通过 |
| 鼠标进入前的折叠标签命中归属 | WindowServer 原生检查通过；展开、收起、移动、隐藏与 6pt 间隙均通过 |
| 折叠标签首次坐标点击 / 双击 | 打开 Passive Quick Card / 进入 Editing 通过；接收面板转发的真实硬件完整路径仍待验收 |
| macOS 原生命中、事件数据、清理与全屏回归 | 29 项通过；包含无关 Carbon 事件交给其他处理器，事件数据检查不投递合成输入 |
| 独立原生接收面板 → WKWebView | 7 项通过：首次点击、更新后的展开坐标、双击、trusted 事件、无键盘焦点和外部前台不变 |
| Quick Card 重复打开 / 关闭 | 100 次通过，关闭后观察 604.74 秒；卡片、8 个接收面板与 6 个进程复用，无焦点拒绝；RSS 增量仍保留，详见 [性能记录](phase-0-macos-performance.md) |
| 透明区域 / 间隙滚轮、真实 Hover 保持外部输入焦点 | 待实测 |
| 系统全局 Control + Option + T | 2026-10-05 真实按键触发 6 次 Carbon 回调，正确捕获外部夹具并进入 KeyboardActive；本轮在编辑前切到终端，完整编辑 / Esc 流程仍待验收 |
| Control + Option + T 独占注册、占用冲突与退出释放 | 通过：运行时另一个独占注册返回 -9878，正常退出后可注册；预先占用时界面和日志显示冲突 |
| 外部应用 → Stack → Quick Card Editing → 两次 Esc | 测试入口下通过：取消编辑后仍在 Sidelet，最终返回外部前台、原窗口和输入框；未重新点击即可输入，不代表精确恢复任意外部应用的指定窗口 |
| 编辑期间主动切换外部应用 | 通过：释放输入状态，不调用前台恢复，不抢回焦点 |
| Mac 原生全屏与所创建的 Space 进入 / 退出 | 已验证隐藏与恢复；普通 Space 切换仍待测 |
| 同一 Space 无边框全屏 | 首次 Overlay 输入识别并隐藏，退出自动恢复；纯键盘进入且鼠标不动时的及时识别仍待测 |
| Mac 多屏、拔插、Dock 变更 | 待实测；双 Stack 在同一物理屏幕上运行不算多屏通过 |
| 10 分钟空闲性能与进程树内存 | 单 / 双 Stack 均完成 600 秒采样；最终双 Stack 平均单核 CPU 0.03543%，RSS 中位数 357.12 MiB；详见 [性能记录](phase-0-macos-performance.md) |

原生启动曾在 AppKit 窗口调整尺寸时崩溃，原因是动态替换对象类型破坏了 Wails 窗口的 KVO 元数据。现保留对象类型，仅对已绑定面板的键盘许可与首次鼠标接收做方法覆盖；未绑定窗口使用原始实现。修复后原生应用可正常启动并持续运行。该实现依赖当前锁定的 Wails / WKWebView 类，升级时需要重新验证。

## 2026-10-04 原生交互复测

Computer Use 已恢复，本轮通过真实 Mac 窗口的可访问性操作、按键、截图和运行日志执行测试。临时下层原生窗口只使用可丢弃的输入框和点击计数器；测试结束后已关闭。最终 Sidelet 恢复到右侧、offset=0.35、44px、8 条假任务和 Passive 状态。

发现并修复了两个实际问题：

1. 键盘打开 Quick Card 使用折叠标签的 14px 锚点，与已经展开的任务行重叠。现读取选中任务行的真实 DOM 矩形；右侧卡片 x=1104，左侧底部卡片 x=305，均位于展开任务的外侧并受工作区约束。
2. 同一张原生卡片从 KeyboardActive 切换到 Editing 时，控制器先调用 Passive 释放焦点，随后 AppKit 返回 `macOS panel refused keyboard focus`。现退出其他窗口的输入模式，保留目标窗口的焦点。复测确认 `KeyboardActive → Editing → KeyboardActive` 各阶段 `key=true`，最终 Esc 后 `key=false`，没有再次出现焦点拒绝。

验证面板的“进入键盘操作”现在会隐藏面板，把操作交给真实 Stack；Esc 会恢复之前的面板。命中诊断可追加 `-trace-pointer`，记录 Overlay 命中进入与离开、原生路由归属与转发计数；默认关闭。

### 旧方案的失败记录

下层测试窗口覆盖当前 WorkArea。透明部分的点击坐标为窗口内 x≈1599.7、y≈183.6；两条折叠标签之间的间隙坐标为 x≈1717.7、y≈201.9，均收到 `mouseDown`。但点击第一条折叠标签内部（x≈1720.3、y≈177.0）时，下层窗口同样收到点击，Sidelet 的命中日志未出现进入事件。

当时不能区分首次点击 / 全局鼠标观察的实现缺陷与工具向指定应用派发事件的影响。部分纯 Overlay 坐标操作还返回 `noWindowsAvailable`，但可访问性按钮操作可用。后续改用预先安装的原生接收区域消除实现中的异步竞态，并补齐窗口标题；复测见下一节，保留此记录用于说明旧方案的问题。

本地证据保存在被 Git 忽略的 `build/results/`：

- `mac-fixture-events.log`：下层点击计数与输入窗口焦点记录。
- `macos-native-validation.log`：带命中诊断的首次点击测试。
- `macos-native-keyboard.log`：修复前的焦点拒绝。
- `macos-native-final-validation.log`：修复后的编辑 / 取消 / Esc 与左右边界。
- `macos-tested.log`：最终构建的 Stack 键盘完成、Undo 和 Passive 恢复。

[Computer Use 技能](/Users/hotstrip/.codex/plugins/cache/openai-bundled/computer-use/1.0.1000717/skills/computer-use/SKILL.md) 明确说明：“press_key and type_text target the specified app, so they cannot invoke global shortcuts.” 所以本轮验证了应用内键盘操作，未将系统全局快捷键标记为通过。多屏、Space、全屏与 10 分钟空闲基线也保留待测。

## 2026-10-04 首次点击修复复测

首次点击路由不再依赖 Hover 通知。接收 NSPanel 预先覆盖标签矩形，渲染窗口保持穿透；Quick Card 则直接接收完整窗口内的事件。快速点击展开时，前端等待 DOM 更新后读取锚点，避免使用旧的折叠宽度定位卡片。

新增 `npm run test:macos`，在 macOS 图形登录会话中创建临时窗口，使用公开的 WindowServer 命中查询进行检查，结束时自动关闭。测试直接引用产品平台实现，19 项断言全部通过：

- 鼠标进入之前，两条折叠标签分别归属对应接收面板；6pt 间隙和渲染透明部分归属下层窗口。
- 展开、收起和移动后命中范围同步更新；隐藏后接收面板不拦截事件。
- 普通查看的渲染窗口和接收面板不取得键盘焦点；接收面板拒绝成为 key window。
- 转发事件数据保持目标窗口、屏幕位置、双击次数、事件类型、修饰键、时间戳和压力。
- 完整 Quick Card 不创建接收面板，窗口自身直接命中。

Computer Use 坐标首次点击打开了 Quick Card，日志显示 `visible=true`、`key=false`；坐标双击进入 Editing，日志显示 `key=true`。标题取消后原内容保留，标题与 22 行备注保存可重新查看；原生滚动能显示第 22 行，底部操作栏保持固定。最新构建复测 `KeyboardActive → Editing → KeyboardActive → Passive` 正常，没有焦点拒绝。测试结束恢复右侧、offset=0.35、44px、8 条假任务与验证面板；临时下层测试窗口已关闭。

测试边界：WindowServer 检查验证了事先安装的命中归属，事件数据断言没有向系统投递输入。Computer Use 向指定应用派发的坐标事件可能直接到渲染窗口，本轮 `forwardedEvents=0`，因此不能据此宣称真实硬件鼠标经过接收面板到 WKWebView 的完整链路已通过。间隙滚轮、真实 Hover 保持外部输入、全局快捷键、多屏 / Space / 全屏与性能基线仍待验收，P0 状态保持 Development Candidate。

本轮证据位于被 Git 忽略的 `build/results/`：

- `macos-hit-regression.log`：19 项原生命中与事件数据断言。
- `macos-hit-final.log`：首次坐标点击、双击编辑和编辑取消 / 保存。
- `macos-hit-final-v2.log`：最新构建的键盘查看、编辑、保存与 Passive 恢复。
- `macos-quick-scroll-bottom.jpeg`：原生卡片长备注末行与固定操作栏截图。
- `macos-hit-build.log`：最终 arm64 `.app` 构建与签名结果。

## 2026-10-04 系统行为与稳定性复测

本轮增加两个独立的测试 `.app`。接收面板夹具直接引用产品平台实现，并仅在夹具进程中开放接收窗口的可访问性，让 Computer Use 明确点击真实的接收 NSPanel。首次点击在 WKWebView 中得到 `(290,34)`、`detail=1`，原生转发计数至少 2；展开后的双击得到 `(250,34)`、`detail=2`，计数至少 6。事件 `trusted=true`，两个面板 `key=false`，外部前台 PID 不变，7 项证据断言通过。这个验证补上了此前 `forwardedEvents=0` 的工具路径缺口，仍不代替真实硬件在各下层应用上的验收。

生产 Stack 也观察到 `forwardedEvents=475` 的 Hover，展开延迟 154ms、短时帧回调约 59fps，接收区域归属匹配且窗口未取得键盘焦点。这是实际回调证据，不是长时间画面 FPS 或丢帧验收。下层夹具在间隙与透明区分别收到 x≈1717.7 和 x≈1599.7 的点击，滚动显示到第 39 行，也能接收输入文字；工具向指定应用派发事件，因此间隙滚轮及完整外部焦点保持仍保留待测。

全屏复测发现两个问题并修复：刘海屏的原生全屏窗口只有 1728×1084，不能要求它覆盖 1728×1117 的整个屏幕；Space 通知又可能在窗口动画结束前到达。现按照屏幕 safe area 判断，并在 Space 通知后等待 800ms 再检查。最终构建的原生全屏测试中，下层夹具保持前台，Sidelet 在屏幕上的窗口数从 9 变为 0，退出后恢复 9。

同一 Space 的无边框全屏不一定产生工作区通知。外部鼠标活动会触发一次延迟检查，Overlay 的输入入口也重新判断；已经识别的全屏会话每秒检查退出。最终构建复测：全屏下首次 Overlay 输入使窗口隐藏，未打开卡片；退出无边框全屏后，无需再点击 Sidelet 即恢复 9 个窗口。普通桌面没有这段定时轮询，Go 和原生测试都覆盖了定时检查的取消与清理。纯键盘进入、鼠标静止和不同视频应用仍需补测。

Quick Card 通过实际键盘操作连续打开、Esc 关闭 20 次，每次回到 Passive；始终复用窗口编号 6447，没有焦点拒绝。两分钟采样中 WebKit 成员没有变更，但内存增加，不能宣称没有泄漏；详细数值和场景边界见 [macOS 性能记录](phase-0-macos-performance.md)。

最新证据位于被 Git 忽略的 `build/results/`：

- `macos-webkit-forwarding.log` / `macos-webkit-forwarding-check.log`：原生接收 → WKWebView 的 7 项检查。
- `macos-system-regression.log`：26 项原生回归，包括刘海全屏几何与延迟检查生命周期。
- `macos-system-go-tests.log` / `macos-system-frontend-tests.log`：8 项 Go 与 10 项前端逻辑检查。
- `macos-system-final-fixture.log` / `macos-system-final.log`：最终构建的全屏隐藏 / 恢复和下层点击、滚动、输入。
- `macos-card-cycles-events.jsonl` / `macos-card-cycles-check.json`：20 次卡片窗口复用与 Passive 恢复。
- `macos-system-build.log`：最终 arm64 `.app` 构建与签名结果。

夹具以 LaunchServices 独立启动。测试检查按钮在动作当时记录 `NSWorkspace.frontmostApplication`、`NSApp.active` 和 WindowServer 窗口边界；不能仅凭 `key=true` 推断系统前台。终端命令可能改变前台，因此 UI 测试期间通过 Node 只读文件查看日志，避免读进程命令干扰判断。

## 2026-10-04 卡片内存专项复测

最终构建完成 100 次实际键盘打开 / Esc 关闭，逐次确认回到 Passive；随后停止界面操作，观察完整十分钟。卡片窗口、Stack 渲染窗口、8 个接收面板和 6 个进程成员身份均未变化，没有焦点拒绝。

恢复阶段末段总 RSS 中位数为 405.42 MiB，比操作前增加 88.86 MiB，主要位于 WebContent。静置期间 RSS 基本持平，WebContent footprint 从关闭后的 100.53 降至 93.13 MiB，最终比基线高 5.77 MiB；GPU footprint 的明显下降还受操作前高初值影响。未发现关闭后继续累积，但尚未定位到确定的泄漏根因，不能把本轮结果写成零泄漏或已修复。

新增逐进程采样 JSONL、按阶段统计工具和 4 项离线分析检查；完整结果、方法、vmmap 分类与复现命令见 [性能记录](phase-0-macos-performance.md)。本轮使用的应用二进制与先前全屏修复构建相同。后续对象诊断另见下一节，P0 保持 Development Candidate。

## 2026-10-04 对象诊断与重复预览渲染

增加 `-memory-dir` 可选诊断：本地请求分别取得 GC 后 Go 堆 / 分配 / goroutine profile、原生弱引用对象统计，以及每个前端视图的组件生命周期、事件订阅和当前 DOM 数。默认运行没有请求轮询、额外诊断订阅或主动 GC。

修复前在同一实例预热 20 次，追加三批各 20 次真实键盘开关，再记录静置快照。原生窗口、WebView、接收面板和订阅没有按操作次数增加；Go 存活堆低于 1 MiB，静置后回落。系统 `heap` 能观察 WebContent 的 malloc 分类，其中暂时增加的 DOM / 事件监听器在静置后回落；这与 RSS 保留并不矛盾，也不能据此承诺完整 JavaScript 堆没有泄漏。

定位到验证面板每次原生卡片打开时也创建一份预览 QuickCard，80 次操作对应 80 次额外创建 / 销毁。原生模式现只显示实际 Quick Card 窗口，浏览器模式保留预览卡片。修复后用相同的 20 次预热加三批 20 次操作复测，验证面板不再创建 QuickCard，实际原生卡片仍复用。

完整对象比较、采样影响、构建哈希和静置结果见 [性能记录](phase-0-macos-performance.md)。本轮减少了已确认的重复组件分配，未宣称解决此前全部 RSS 增量；全局快捷键、多屏和真实鼠标验收继续保留。

## 2026-10-04 快捷键注册与外部焦点

锁定的 Wails beta.27 使用 Carbon 非独占注册。SDK 明确说明非独占注册可能被其他应用的独占绑定压制，注册成功不能代替按键交付。改为原生 `RegisterEventHotKey`，使用 `kVK_ANSI_T`、`controlKey | optionKey` 与 `kEventHotKeyExclusive`；事件 ID 匹配后才进入产品控制器，无关事件返回 `eventNotHandledErr`。注册在 AppKit 线程执行，绑定窗口关闭时释放。依据为本机 Command Line Tools SDK 的 `CarbonEvents.h` 对 `kEventHotKeyExclusive` 的定义及锁定依赖源码，没有修改 Wails 模块缓存。

注册探针不生成或投递输入。本机实测：应用未运行时独占注册和释放均返回 0；新应用运行后探针返回 `eventHotKeyExistsErr=-9878`；使用应用正常 Quit 退出后又可注册。另一进程先独占 20 秒时，Sidelet 启动报告 `Ctrl+Option+T registration failed: RegisterEventHotKey OSStatus=-9878 (shortcut already in use)`，验证面板也显示该错误。占用解除后需要重启，当前不自动重试。

新增 `-interaction-test`，仅在显式开启时接收前台一次性夹具的测试请求，校验夹具 bundle ID 和系统前台 PID；请求日志为 `source=fixture-request`，与真实 Carbon 回调的 `source=global-shortcut` 区分。测试窗口记录结构化的系统前台、应用激活、窗口 key、输入框 first responder 与输入变化。默认运行没有该测试接收入口和焦点边界日志，不增加轮询。

这个入口暴露了之前从验证面板进入无法覆盖的外部激活竞态：`activateIgnoringOtherApps` 尚未完成时立即读取 `isKeyWindow`，代码会将面板重新设为 Passive；随后工作区通知又会按旧的 key-window 状态提前退出键盘模式。现保留待激活状态，在 `NSApplicationDidBecomeActiveNotification` 时设置 key window 与 first responder。Passive 会取消待激活状态，前台归属判断同时检查系统前台 PID，避免外部应用已经激活而 Sidelet 的 key-window 属性仍旧时抢回焦点。AppKit 通知观察随绑定窗口清理；每个绑定窗口新增一个激活观察，不使用忙等或定时重试。[NSRunningApplication 文档](https://developer.apple.com/documentation/appkit/nsrunningapplication?language=objc)也说明相关动态属性存在竞态，并在主事件循环更新。

最终焦点轮 Sidelet PID 986、夹具 PID 1672，完成三种流程：

1. 外部输入框 → 测试入口 → Stack ↓ / Enter → Quick Card E → 输入未保存标题 → Esc。原标题恢复，Quick Card 保持 KeyboardActive，系统前台仍为 Sidelet；第二次 Esc 请求恢复夹具。
2. 外部输入框 → 测试入口 → Stack Esc，直接恢复夹具。
3. Quick Card Editing → 点击夹具的 Activate test app，Sidelet 释放输入状态，未调用前台恢复；夹具保持前台并可输入。

第一种流程在任何再次点击或输入前只读夹具日志，确认 `foregroundPID=1672`、`appActive=true`、`windowKey=true`、`inputFocused=true`。随后没有点击输入框，直接输入 `-after-escape`，记录的系统前台与输入焦点仍正确。最终轮没有焦点拒绝，12 项证据检查均通过。外部恢复仍采用激活应用的系统 API，这个单窗口夹具的结果不等于任意多窗口外部应用都能精确恢复指定窗口。

复现命令见 [README](../README.md#macos-快捷键与外部焦点复测)。本地证据位于 `build/results/`：

- `macos-shortcut-{free-before,occupied,free-after,holder}.log`：独占注册 / 释放与临时占用。
- `macos-focus-conflict.log`、`macos-focus-conflict-ax.txt`：冲突日志和可见错误。
- `macos-focus-app.log`、`macos-focus-fixed-app.log`：保留外部切换失败与中间修复失败，未将它们计为通过。
- `macos-focus-final-{app,fixture}.log`、`macos-focus-before-resumed-input.log`、`macos-focus-first-escape-ax.txt`、`macos-focus-summary.json`：三种最终焦点流程、恢复后先只读的记录与证据检查。
- `macos-focus-final-native.log`、`macos-focus-go-tests.log`、`macos-focus-final-build.log`：原生回归、Go race 与构建记录。

最终打包后二次确认：Sidelet PID 3670、夹具 PID 4029，再走一遍外部输入 → Stack → Quick Card Editing → 两次 Esc；恢复后先只读确认前台 / key window / 输入焦点，再输入 `final-after-escape`，全部通过。最终二进制 SHA-256 为 `438a8e56fd7b0ae5df138afc0082fdfe7637f60d37717e384c62c6491a3ba42f`。对应证据为 `macos-focus-reviewed-{app,fixture}.log`、`macos-focus-reviewed-before-input.log`、`macos-focus-reviewed-first-escape-ax.txt`；最终 29 项原生回归、12 项前端测试、Go race、Windows amd64 / arm64 交叉编译均通过，见 `macos-focus-reviewed-{native,frontend,go}.log` 与 `macos-focus-reviewed-build.log`。退出测试窗口后恢复普通 `-main` 实例，未启用测试通知入口；默认数据和状态保留 8 条任务 / Passive。

Computer Use 的 `press_key` / `type_text` 向指定应用操作，不能触发系统全局快捷键；本轮验证的是注册生命周期和测试入口后的产品焦点流程。实际物理 Control + Option + T、真实鼠标在不同下层应用的完整路径、多屏 / 拔插仍待验收，P0 保持 Development Candidate。

## 2026-10-04 实机验收准备

用户选择先准备真实键鼠验收，继续遵守 Revision 4 的 Spike 门槛；本轮未接入 SQLite，也未升级 P0 状态。新增独立会话启动器和检查器，命令见 [README](../README.md#准备最后一轮真实键鼠验收)：

- `start-macos-acceptance.py` 拒绝已有项目实例和复用的输出目录，记录两个二进制哈希、PID、时区、系统与显示器信息；等待真实视图和快捷键注册就绪，再打开中文步骤夹具。
- `-trace-focus` 单独开启焦点日志，不开启测试键盘通知入口；兼容原有 `-interaction-test`。夹具的 manual 模式隐藏测试入口，记录事件时间、前台、first responder、屏幕点击坐标与真实滚动边界变化。滚轮的移动可能异步发生，已用 clip-view bounds 通知采集，不再立即读取后误判为零移动。
- `check-macos-acceptance.py` 只认可 Carbon 回调的 `source=global-shortcut`，核对编辑 / 取消的系统前台、Esc 后先恢复再输入，以及期间是否重新点击或激活夹具。间隙与透明区按事件发生时的原生命中矩形分类；诊断模式现在记录区域更新，避免使用启动时的旧矩形。滚动完成时间与事件发生时间分开，几何使用后者。
- `--manual-input-confirmed` 是操作者对输入来源的声明，不从 trusted、注册成功或应用内测试入口推断硬件输入。报告只有当前输入场景的 pass / fail / pending，退出码分别为 0 / 1 / 2，不自动升级 P0。详细跟踪会增加采集开销，这种会话不能用作正常空闲性能基线。

9 项离线检查通过，包括错误入口、缺失真实输入声明、混用进程日志、陈旧时间字段、仅注册无交付、恢复后重新点击、陈旧 key-window 状态、Overlay 之外的点击 / 无位移滚轮，以及异步滚动使用事件发生时的几何。macOS 构建和签名、Go race、Windows amd64 / arm64 交叉编译通过。

启动器已实际验证拒绝已有实例；夹具 UI 的中文步骤和隐藏测试按钮确认正常。Computer Use 自测记录了输入、点击坐标及滚动 0 → 785，这只是采集工具测试，不算真实硬件验收。自测会话 `macos-manual-khnl47nz` / `macos-manual-0fhqu2hk` 已关闭，证据为 `macos-acceptance-smoke-summary.json`；检查器仍保留待验收。

最后另开干净会话 `build/results/macos-manual-xp6roc2z/`，Sidelet PID 15903、夹具 PID 15924。本机枚举到 1 块 1728 × 1117、backingScale=2 的显示器。二进制 SHA-256 为 `22f10d185dac010496302d11354f20243b830cbaea6956cc8db4c3d927ffccde`；会话中的 `session.json`、`GUIDE.md`、两个日志和 `report.json` 保留环境、步骤与待验收结果。未操作时检查器正确返回 2，不将注册成功标为交付通过。最终构建、Go race 和 9 项检查器测试日志分别为 `macos-acceptance-final-build.log`、`macos-acceptance-final-go-tests.log`、`macos-acceptance-tool-tests.log`。

真实 Control + Option + T 及鼠标路径须由用户在该窗口操作；测试完可让助手只读日志并运行检查器。Passive 卡片完整交互、其他下层应用、普通 Space、多屏 / 拔插 / Dock 仍保留待验收。

## 2026-10-04 当前构建自动复测

用户要求继续由助手自测。本轮使用相同的最终二进制 `22f10d185dac010496302d11354f20243b830cbaea6956cc8db4c3d927ffccde`，通过 Computer Use 自动操作原生窗口。先关闭此前待手动会话 `macos-manual-xp6roc2z`，另开自动测试实例 Sidelet PID 17190、一次性外部夹具 PID 17260。键盘入口明确来自 `fixture-request`，没有将其冒充 Carbon 全局按键。

9 项 UI 证据检查全部通过：取消编辑保留原标题、Editing → KeyboardActive、第二次 Esc 后先只读确认外部前台与输入焦点、未重新点击即可继续输入、主动切换应用不调用恢复且可输入、下层滚动出现真实位移、原生全屏隐藏 / 恢复，以及全程无焦点拒绝。普通状态包含验证面板，共 10 个本应用屏幕窗口；全屏时为 0，退出恢复 10。UI 工具曾短暂断连，重新读取状态和日志后继续完成，没有将失败的工具调用视为通过。

29 项原生回归全部通过。独立路由夹具 PID 18929 通过真实接收面板转发点击到共享 WKWebView：折叠首次点击坐标为 (290,34)、转发计数 2；展开双击为 (250,34)、计数 6。两个面板均未获得 key，外部前台始终为 PID 17260，7 项检查通过。这是指定应用的工具事件路径，不代替物理鼠标在各下层应用上的完整系统分发验收。

证据位于 `build/results/macos-selftest-20261004/`：`summary.json`、`sidelet.log`、`fixture.log`、`before-resumed-input.log`、取消与全屏 AX 记录、`native-regression.log`、`routing.log` 和 `routing-check.log`。汇总明确将真实全局快捷键、物理鼠标和多个物理显示器测试标为 false。自测和路由夹具均已关闭，Sidelet 恢复普通 `-main`、8 条任务、Passive，无测试通知入口或跟踪开关。

## 2026-10-05：真实按键尝试与 UI 队列死锁修复

用户在手动会话 `macos-manual-klz8nrnb` 中报告已实按快捷键，但日志没有 Carbon 回调。Sidelet PID 21255 仍存在，窗口状态读取超时，前台变化也不再记录；3 秒线程采样中主线程停在 Go 的条件变量等待。该会话不能计为快捷键通过，证据保留在 `sidelet.log`、`fixture.log` 和 `sidelet-hang-sample.txt`。

代码排查发现可确定复现的死锁：原生回调在 UI 线程向容量 128 的 channel 写入，消费端在 `InvokeSync` 等待 UI 线程。未消费时连续发出 512 条消息，旧实现在第 129 条阻塞，回归测试失败。该条件与现场挂起相符；线程采样没有完整 Go goroutine 栈，不能断言它是此次挂起的唯一原因。

改为带互斥锁的 FIFO 和容量 1 的唤醒通知：入队不等待消费端，事件不丢失，处理仍在 UI 线程逐条执行。消费后清除事件引用，清空或压缩已消费的存储；没有添加空闲轮询或每条事件的 goroutine。新增回归覆盖 512 条突发、8 个并发生产者的 4096 条有序事件、唤醒及关闭。原有全屏计时器检查适配新队列，Go race 全部通过；macOS 构建、签名以及 Windows amd64 / arm64 交叉编译通过。启动器补充标准输出和标准错误文件，供后续捕获运行时诊断。

旧实例无法处理正常退出信号，仍占用快捷键。核对可执行文件、参数和 PID 后强制结束，只涉及内存假数据。随后自动复测实例 PID 70468 / 夹具 70976，通过编辑取消保留原标题、恢复后先只读确认外部焦点、无点击继续输入，以及原生全屏时 10 → 0、动画结束后 0 → 10 的窗口隐藏 / 恢复。这轮键盘入口为 `fixture-request`；它启动时因旧实例尚未退出而注册冲突，因此只算交互回归，不算真实快捷键验收。

证据：`macos-manual-klz8nrnb/queue-before.log`、`macos-queue-fix-go-tests.log`、`macos-queue-fix-build.log`、`macos-queue-fix-acceptance-tests.log`，以及 `macos-queue-fix-selftest/` 中的日志、AX 记录、`before-resumed-input.log` 和 `summary.json`。5 项原生窗口交互检查通过，9 项验收检查器测试通过。

复测实例正常退出后，已启动干净手动会话 `macos-manual-20261005-queue-fixed`，Sidelet PID 72040、夹具 PID 72061；独占快捷键注册成功，没有测试键盘入口。新二进制 SHA-256：`02e75d9012455344529de2ad3b3684a22bc1e16633e4226bffe4b67635821142`。真实按键复测、睡眠 / 唤醒后的持续响应、物理鼠标及多屏场景仍待验收，P0 保持 Development Candidate。

## 2026-10-05：真实全局快捷键回调通过

在上述修复版手动会话中，用户实按 Control + Option + T，00:20:26.131281 起记录到 6 次 `keyboard request source=global-shortcut`。首次回调时外部前台是夹具 PID 72061；捕获 `ownWindow=0 / pid=72061`，随后系统前台切到 Sidelet PID 72040，Stack key window 为 8257、first responder 为 WKWebView。没有启用 `interaction-test`。这证明真实全局入口交付与激活通过。

00:20:31 用户切到终端 PID 1402，Sidelet 正常释放键盘状态，日志 `exit-modes` 的 key window 为 0、appActive=false，没有请求恢复外部前台。用户没有在这次键盘会话中完成编辑 / 两次 Esc，因此该组合流程继续待验收；上一节的 5 项自动交互回归单独保留。

发现检查器只截取最后一次快捷键之后的日志，连续按键复用首次焦点捕获时会误报捕获待验收。现只在同一次交互内回溯到首次捕获；退出、恢复、其他键盘入口或新的焦点捕获会分隔交互。新增重复按键、退出后的新入口、错误新捕获与其他入口的回归检查，合计 13 项通过。重新生成的 `report.json` 中真实 Carbon 入口、外部焦点捕获和入口前输入均为 pass；其余未执行项保持 pending。

新增证据位于手动会话的 `global-shortcut-evidence.log`、`global-shortcut-summary.json`、`report.json`，脚本检查结果为 `macos-global-repeat-acceptance-tests.log`。会话基线输入来自工具，真实快捷键来自用户，输入来源记录为混合；未声明整个会话都使用真实键鼠。已准备后续 Hover 输入和第三 / 第四条标签间隙及左侧透明区域的点击、滚轮步骤。

真实快捷键后的完整编辑 / Esc、物理鼠标、睡眠 / 唤醒、其他下层应用、普通 Space、多屏 / 拔插 / Dock 仍待验收。P0 保持 Development Candidate。

## 原生验收步骤

1. 在编辑器输入文字，同时 Hover 右侧任务；约 150ms 展开，编辑器继续接收文字与方向键。
2. 分别在任务间隙、任务周围点击和滚动，确认下层编辑器与浏览器接收事件；测试快速移入后的首次点击。
3. 点击任务打开 Quick Card；移动到卡片不提前收起，离开两者 500ms 后关闭。测试完成、Undo、Snooze、备注编辑与取消。
4. 从编辑器按 Control + Option + T，测试 ↑ / ↓、Space、S、E、Enter 与 Esc。编辑时离开卡片应保持编辑，退出后恢复前台应用。
5. 验证左右边缘、顶部和底部位置、小工作区 `+N`、卡片内容滚动；连接不同 Retina 比例显示器并拔插，改变 Dock 位置与自动隐藏设置。
6. 验证普通 Space 切换、原生全屏和同一 Space 内的视频全屏。普通桌面不轮询，已识别的全屏会话每秒检查退出；纯键盘进入无边框全屏且鼠标不动可能缺少进入通知，需要记录漏判。Dock 自动隐藏时，普通最大化窗口也可能被几何启发式识别为全屏，需要补测。
7. 省略 `-main` 运行 10 分钟，记录原型与 WebKit 子进程 CPU / 内存；反复打开关闭卡片，检查窗口复用和稳定性。

命中、焦点和系统行为通过且建立可信性能基线之后，再升级为 Development Baseline。
