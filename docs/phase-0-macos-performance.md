# Phase 0 · macOS 性能记录

2026-10-04，在本机建立第一轮数据。状态仍为 **Development Candidate**，这些数字不代表完整 P0 / P2 通过。

## 环境与口径

- macOS 26.6.2（25G83），Apple M3 Max / arm64，14 个逻辑核，36 GiB 内存。
- 单块 Retina 屏幕：1728×1117 逻辑点，工作区 1728×1007、顶部 33，backingScale=2。
- Go 1.27.1，Wails v3.0.0-beta.27，Svelte 5.57.1；本地临时签名的生产构建。
- 空闲场景通过 `open -n` 启动独立应用实例，省略 `-main` 与 `-trace-pointer`，8 条内存假任务，Quick Card 未打开；预热 30 秒，再每 5 秒采样 600 秒。
- CPU 包含 Sidelet 主进程及归属的所有 WebKit 进程。100% 表示占满一个核，归一化值再除以 14；平均值按实际采样间隔加权，首个样本没有 CPU delta，峰值是采样区间平均值。
- 内存使用 MiB（2²⁰ 字节）。RSS 求和可能重复计算共享页；physical footprint 是另一种系统资源计量，包含被计入该进程的压缩等内存，不等于 Windows Private Bytes，也不能和 RSS 相加。

WebKit 的辅助进程由系统启动，不能只沿 PPID 查找。采样器同时匹配 resource 与 jetsam coalition ID，并校验进程启动时间以防 PID 复用。单 Stack 包含主进程、3 个 WebContent（Stack、隐藏验证面板、隐藏 Quick Card）、GPU 和 Networking，共 6 个进程；双 Stack 多一个 WebContent，共 7 个。接收 NSPanel 不增加 WebView。

coalition 查询使用 [Apple XNU 的私有诊断 ABI](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/proc_info_private.h)，仅在本地采样工具中使用，不进入应用。接口不兼容会使采样失败；从终端直接启动的多个应用可能共享 coalition，所以推荐通过 LaunchServices 启动独立实例。[XNU 资源计量实现](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_resource.c)中的 CPU 时间经 `mach_timebase_info` 转成纳秒后计算，已与本机进程 CPU 读数交叉核对。

## 十分钟空闲

| 指标 | 单 Stack：全屏修复前 | 双 Stack：最终全屏修复构建 |
|---|---:|---:|
| 实际时长 / 样本数 | 600.02 秒 / 121 | 600.02 秒 / 121 |
| 归属进程数 / WebContent 数 | 6 / 3，全程不变 | 7 / 4，全程不变 |
| CPU 加权平均，单核口径 | 0.04590% | 0.03543% |
| CPU 加权平均，14 核归一化 | 0.00328% | 0.00253% |
| CPU 中位数 / 峰值，单核口径 | 0.00579% / 1.99685% | 0.00667% / 2.22603% |
| 主进程 RSS 中位数 | 104.80 MiB | 115.98 MiB |
| WebKit RSS 中位数 | 194.88 MiB | 241.14 MiB |
| 总 RSS 中位数 / 最大值 | 299.59 / 300.48 MiB | 357.12 / 357.36 MiB |
| 总 RSS 首值 → 末值 | 298.98 → 300.30 MiB | 355.39 → 357.19 MiB |
| 总 footprint 中位数 / 最大值 | 126.68 / 126.90 MiB | 154.30 / 155.22 MiB |
| 总 footprint 首值 → 末值 | 126.32 → 125.85 MiB | 155.22 → 154.35 MiB |

单 Stack 的平均 CPU 低于 Revision 4 的首轮 0.5% 预算，但两段 5 秒样本超过 0.5%；峰值包含在统计中，没有剔除。采样后半段曾操作独立的屏幕中部测试夹具，Sidelet 保持折叠 Passive，未打开卡片；这不是整个桌面完全无人操作的实验，不能把峰值归因于某一动作。进程成员和 footprint 未持续增长，不等于证明没有内存泄漏。

双 Stack 完整采样中平均 CPU 同样低于首轮预算，但单核口径的峰值为 2.22603%，未剔除。总 RSS 增加约 1.80 MiB，footprint 下降约 0.88 MiB，进程成员全程未变。十分钟内没有持续明显的内存增长，仍不能据此证明长时间无泄漏。

双 Stack 在同一物理显示器上左右分布，各 4 条任务，测试夹具已关闭；采样期间做了一次只读界面查看和 WindowServer 布局核对，没有点击、Hover 或编辑 Sidelet。窗口核对得到 2 个渲染 NSPanel 与 8 个接收 NSPanel，隐藏的验证面板和 Quick Card 不在屏幕上。单 Stack 和双 Stack 来自不同构建，数字只能作为各自场景基线，不能直接把差值归因于额外 Stack。

可执行文件 SHA-256：

- 单 Stack：`9f4421b1df289da037d767f017170c70f5f3b864bd15abb6b9f4e69f90b72bc2`。
- 双 Stack / 最终全屏修复：`6e882a68418d81f064426280bf2f4803cbe9aa4da76f5516ae33e212fc1669bb`。

## 卡片重复打开与全屏检查

实际 WKWebView 操作连续打开、Esc 关闭 Quick Card 20 次，全部回到 Passive，始终复用原生窗口编号 6447，没有焦点拒绝。期间 120.02 秒采样得到 119 个样本，6 个归属进程没有更换。该场景启用了验证面板和命中追踪，包含主动交互和动画，不是空闲 CPU 测试。

| 卡片场景指标 | 结果 |
|---|---:|
| CPU 加权平均 / 峰值，单核口径 | 5.39085% / 31.21868% |
| 总 RSS 首值 → 末值 | 341.67 → 371.00 MiB |
| 总 footprint 首值 → 末值 | 142.46 → 162.05 MiB |
| 总 footprint 采样峰值 | 449.68 MiB |

这轮重复操作没有增加窗口或 WebKit 成员，但内存有所增加。最后一次关闭在 16:48:52，采样在 16:48:56 结束，关闭后只观察了约 4 秒，未验证内存回落。尚未区分 WebKit 缓存、动画资源与泄漏；短测试不能据此承诺长时间稳定，也不能把窗口复用写成内存无增长。需要更长的重复 Hover / 卡片采样和性能分析。

最终构建另采集了 60.03 秒的全屏进入、停留与退出过程。整个过程包含 UI 操作，因此不把整段平均值列入空闲表。依据产品日志的 `fullscreen=true/false` 时间，选取距两次切换至少 1 秒、完全位于已识别全屏内的采样区间，共 17 段 / 17.31 秒：单核 CPU 加权平均 0.17156%，区间峰值 0.43192%。它仅用于观察全屏期间 1Hz 退出检查的短期开销，不是十分钟全屏空闲验收。正常空闲不启用该检查，退出后停止。

卡片测试构建 SHA-256 为 `0972d4b279e7c16cc2e3acb989f396566d287ae08c6f8e3e7146249842bef1b3`，早于最终的全屏退出定时检查修改。

## 100 次卡片开关与十分钟恢复（2026-10-04）

本轮使用最终全屏修复构建（SHA-256 `6e882a68418d81f064426280bf2f4803cbe9aa4da76f5516ae33e212fc1669bb`），新启动单 Stack 实例 PID 63059，参数 `-main -trace-pointer`。通过 Computer Use 从验证面板进入键盘模式、Enter 打开同一条任务、Esc 关闭；不编辑或保存内容。每次读取实际界面，确认卡片打开并回到 Passive，100 次全部成功。

预热 30 秒后每秒记录各进程，操作前另有超过 60 秒的基线。操作持续 404.13 秒；最后一次关闭为 17:28:12.967，观察到 17:38:17.707，覆盖 604.74 秒。采样正常结束，共 1091 个样本 / 1113.95 秒；主进程与 3 个 WebContent、GPU、Networking 的成员身份全程不变。卡片窗口编号始终为 6784，Stack 渲染窗口为 6785，8 个接收面板也复用，没有焦点拒绝。

下表每个单元为 **RSS / footprint（MiB）**，分别使用操作前最后 30 秒、关闭后最初 30 秒、关闭后第 570～600 秒的中位数；三个窗口各有 29 个样本。主进程包含 AppKit / Wails，不能将其总量等同于 Go 堆。

| 分类 | 操作前 | 关闭后 0～30 秒 | 关闭后 570～600 秒 |
|---|---:|---:|---:|
| Go 主进程（含原生宿主） | 116.30 / 36.14 | 128.70 / 43.31 | 128.62 / 43.13 |
| WebContent × 3 | 140.50 / 87.36 | 211.70 / 100.53 | 211.61 / 93.13 |
| GPU | 43.94 / 213.78 | 49.28 / 17.19 | 49.28 / 17.19 |
| Networking | 15.83 / 5.58 | 15.91 / 5.42 | 15.91 / 5.42 |
| 总量 | 316.56 / 342.86 | 405.55 / 166.41 | 405.42 / 158.86 |

RSS 最终比基线增加 **88.86 MiB**，其中 WebContent 增加 71.11 MiB、主进程增加 12.33 MiB。静置期间总 RSS 只减少约 0.13 MiB，没有明显回落，也没有继续增长。WebContent footprint 从关闭后的 100.53 降到 93.13 MiB，最终较基线增加 5.77 MiB；主进程 footprint 最终较基线增加 6.98 MiB。

GPU 的基线 footprint 本来就高达 213.78 MiB，交互期间也有明显波动，静置后为 17.19 MiB。因此总 footprint 从 342.86 降到 158.86 MiB，主要受 GPU 的高初值影响，不能把这个下降当作卡片释放了全部内存或没有泄漏的证据。第 20 / 40 / 60 / 80 / 100 次附近的总 RSS 分别为 366.53 / 378.97 / 385.89 / 393.64 / 407.09 MiB；这些单点含动画与界面工具的瞬态，未用它们替代恢复阶段的中位数。

静置阶段不再访问 Sidelet 界面，保留验证面板；只读采样期间额外执行了一次主进程和一个 WebContent 的 `vmmap -summary`。第 30～600 秒内加权平均单核 CPU 为 0.01763%，区间峰值 1.98144%，没有剔除。这是开启验证面板与诊断的恢复场景，不能直接代替正常隐藏面板的空闲预算。界面工具在交互阶段频繁读取可访问性状态与截图，结果也可能含观察工具影响。

17:30 的 vmmap 快照中，WebContent PID 63064 的 footprint 为 49.8 MiB、峰值 128.2 MiB；WebKit malloc 区域 resident 为 46.6 MiB、allocated 为 21.8 MiB，还包含图形区域。主进程 footprint 为 43.2 MiB，DefaultMallocZone 的 allocated 为约 8.5 MiB、碎片区域约 12 MiB。它们是单时刻的分类，不能推断哪些业务对象泄漏。[Apple 的 vmmap 说明](https://developer.apple.com/library/archive/documentation/Performance/Conceptual/ManagingMemory/Articles/VMPages.html)区分虚拟保留空间与实际页面占用；[WebKit 的内存诊断文档](https://docs.webkit.org/Infrastructure/MemoryInspection.html)也分别列出 dirty、clean 与 reclaimable，不能仅凭 RSS 判断活跃对象数量。

结论：本轮确认窗口 / 进程复用稳定，停止交互后没有继续积累，GPU 和部分 WebContent footprint 会回落；RSS 仍保留较大的增量。**尚未定位到确定的泄漏根因，不能宣称零泄漏或问题已修复。** 本轮交付的是更完整的采样与分析工具。下一轮可在已预热的同一实例上比较多个操作批次，并结合对象 / 分配堆分析区分缓存、空闲页保留和泄漏；编辑长备注及鼠标 Passive 查看仍需各自的稳定性测试。

统计工具输出逐进程 JSONL、分组时间序列 CSV 和阶段中位数；拒绝不足 100 次的记录或未覆盖完整十分钟的数据。4 项离线检查覆盖恢复窗口、交互峰值隔离、PID 复用和失败 / 缺失循环；采样器的逐进程汇总与结束标记也通过实际小测试。工具改动没有改变被测应用二进制。

本轮证据位于 `build/results/`：

- `macos-card-memory-100-20261004-171912.{csv,processes.jsonl,environment.json,summary.json}`。
- `macos-memory-100-events.jsonl` / `macos-memory-100-phases.json` / `macos-memory-100-check.json`。
- `macos-memory-100.analysis.json` / `macos-memory-100.series.csv` / `macos-memory-100-analysis.log`。
- `macos-memory-100-app.log` / `macos-memory-100-collector.log`。
- `macos-memory-100-{go,webcontent}-vmmap.txt`。

复现分析（先取得同样格式的 UI 成功记录与阶段时间）：

```sh
python3 scripts/analyze-macos-card-memory.py \
  --samples build/results/macos-card-memory-100-20261004-171912.processes.jsonl \
  --events build/results/macos-memory-100-events.jsonl \
  --phases build/results/macos-memory-100-phases.json \
  --output build/results/macos-memory-100
python3 scripts/test-macos-memory-analysis.py
```

## 对象诊断与分批复测（2026-10-04）

新增显式 `-memory-dir` 诊断。每次本地请求保存 Go 的 heap / allocs / goroutine profile、强制 Go GC 后的 MemStats、原生弱引用对象计数及所有前端视图的组件 / 注册 / 当前 DOM 计数。只有开启该参数才使用 1Hz 请求轮询、64 KiB Go 内存采样率、原生弱引用注册表和额外前端订阅。快照的 GC、JSON / profile 写入、系统堆检查和界面观察都会改变内存或时序，不能将本轮 CPU / footprint 峰值作为正常空闲预算。[Go pprof 文档](https://pkg.go.dev/runtime/pprof)说明 profile 是采样分配统计；Go 堆不包含全部 AppKit 或 WebContent 内存。

两个构建分别新启动单 Stack，使用 `-main -memory-dir`，不启用命中追踪。每个实例通过实际界面完成 20 次预热，再追加三批各 20 次 Enter 打开 / Esc 关闭，分别保存冷启动、20 / 40 / 60 / 80 次和静置快照。两轮各 80 次全部成功，归属的 6 个进程身份均未变化；对象诊断轮的实际静置快照距最后关闭 119.72 秒，修复后为 130.92 秒。它们是约两分钟的对象定位，不能替代此前完整十分钟恢复实验。

定位到一个明确的重复分配：原生 Quick Card 打开时，隐藏的验证面板还会挂载一份预览 QuickCard。修复前 80 次操作对应这份组件创建 / 销毁各 80 次；实际原生 QuickCard 在第一次选中任务时重建一次，之后复用一个存活实例。现在原生验证面板不再挂载这份预览卡片，浏览器预览仍保留。修复后 80 次操作中验证面板的 QuickCard 创建数为 0，实际原生卡片行为正常。

| 对象 / Go 指标 | 修复前 | 修复后 |
|---|---:|---:|
| 原生附着 WebView 数 | 3，全程不变 | 3，全程不变 |
| 接收面板累计创建 / 存活 | 8 / 8，全程不变 | 8 / 8，全程不变 |
| 接收视图 / tracking area 数 | 8 / 8，全程不变 | 8 / 8，全程不变 |
| 原生窗口 observer / workspace observer / mouse monitor | 6 / 2 / 2 | 6 / 2 / 2 |
| 每个前端视图的桥接订阅 / document 或 window 监听 / ResizeObserver | 8 / 3 / 1，不变 | 8 / 3 / 1，不变 |
| 隐藏验证面板 QuickCard 累计创建 / 关闭后 live | 80 / 0 | 0 / 0 |
| Go GC 后 HeapAlloc，20 → 40 → 60 → 80 次（KiB） | 809.24 → 902.45 → 922.81 → 962.05 | 799.71 → 863.77 → 878.23 → 901.16 |
| Go GC 后 HeapAlloc / HeapObjects，静置 | 844.32 KiB / 2587 | 830.63 KiB / 2590 |
| Go goroutine 数 | 13，全程不变 | 13，全程不变 |

前端 live 是组件生命周期的挂载计数，不是 JavaScript 引擎的全部存活对象；当前 DOM 计数也不包含已脱离文档的节点。原生统计使用弱引用，观察对象而不延长其生命周期；WebView 数只覆盖当前附着视图。原生夹具中关闭的两个面板仍曾出现在 AppKit 自动释放池里，应用持有列表和父子窗口链接已清空。保存的只读内存图支持区分这些路径，但单次延迟释放不足以证明泄漏或证明所有对象最终释放，因此本轮没有盲目更改 NSPanel 生命周期。

系统 `heap` 在本机能够取得 WebContent 的 malloc 分类，无需完整 Xcode。以下是两个有明显短期变化的进程，保留 PID 标识；尚未可靠建立 WebContent PID 与三个业务视图的对应关系。分类只覆盖系统 malloc 区域和识别出的原生类型，不是完整 JavaScript 堆或业务对象引用链。

| 指标，60 次 → 80 次 → 静置 | 修复前 WebContent 79965 | 修复后 WebContent 84295 |
|---|---:|---:|
| malloc allocated（MiB） | 13.67 → 15.66 → 13.74 | 26.93 → 31.54 → 23.50 |
| WebCore::JSEventListener | 59 → 75 → 59 | 77 → 117 → 67 |
| WebCore::HTMLButtonElement | 28 → 48 → 28 | 33 → 53 → 28 |
| WebKit::PlatformCALayerRemote | 19 → 19 → 19 | 19 → 19 → 19 |

修复前第三批增加约 1.99 MiB malloc 分配，静置回落约 1.92 MiB；修复后也仍存在暂时增长，随后回落。另一个 WebContent 的监听器为 56 → 58 → 56、按钮为 10 → 11 → 10；其余一个进程为 26 个监听器和 8 个按钮，均保持不变。两轮三个 WebContent 的 Remote layer 数分别一直为 12 / 13 / 19。修复后仍有验证面板标签预览更新和引擎回收延迟，不能把减少 QuickCard 挂载等同于所有 DOM 分配消失。

GPU vmmap 中，修复前 graphics 区域 resident 从关闭后的 35.8 MiB 降至静置的 3840 KiB，区域数从 26 降至 22；修复后从 51.8 MiB 降至 3840 KiB，区域数从 28 降至 22。此类图形区域大小与 footprint 计量不同，不能相加。两轮静置时 GPU footprint 约 17 MiB。

按 Go 快照附近的最近单个采样点，总 RSS 在 20 次预热 / 80 次关闭 / 静置时分别为修复前 **356.95 / 382.19 / 380.59 MiB**、修复后 **356.59 / 397.08 / 395.78 MiB**。修复后的 RSS 并未更低；两轮的引擎预热、观察工具和阶段暂停也有差异，不能用它们宣称节省了多少 MiB。确认的收益是消除每次操作的一份完整预览卡片创建；Go 存活堆、原生对象、已识别的部分 DOM / 监听 / 图形资源均有界或出现回落，RSS 保留问题仍没有完整对象级根因，P0 不据此升级为通过。[Apple 的堆分析方法](https://developer.apple.com/videos/play/wwdc2024/10173/)也区分持久分配与持续创建 / 销毁，完整引用链仍需进一步定位。

对象诊断修复前 SHA-256：`e6b64e6ec9e15eb200fd809308507d66d03391aa88c7f9b3c9ff847e955b36fd`；修复后：`ec38b7728e9cb87faa8effb5880a4464235391111d53c43b2432d6fbdbff0bb4`。前者包含诊断但尚未移除重复预览，二者均不同于此前无对象诊断的十分钟基线构建。

本轮验证：Go race 测试通过，前端 12 项通过，Svelte / TypeScript 0 错误和 0 警告，原生 28 项路由 / 数据 / 全屏 / 对象诊断断言通过，堆分析器 4 项离线检查通过；Mac arm64 构建 / vet / 临时签名校验和 Windows amd64 / arm64 交叉编译通过。Go 新检查覆盖非法 / 重复快照标签和可读取的 profile，前端新检查覆盖重复销毁与快照隔离，堆解析检查覆盖分类变化、错误报告、重复布局汇总及同名类型跨二进制区分。

主要证据位于 `build/results/`：

- `macos-object-run/`、`macos-object-after-run/`：分阶段 Go profile / JSON、原生 / 前端 JSON、80 次 UI 记录和阶段时间。
- `macos-object-batches-20261004-175654.*`、`macos-object-after-batches-20261004-180619.*`：逐进程样本、环境和结束统计。
- `macos-object-comparison.json` / `.log`，复现脚本 `compare-object-runs.py` 验证两轮 80 次记录、完整快照、进程身份稳定和组件计数。
- `macos-object{,-after}-webcontent-{60,80,idle}-*-heap.log`、GPU vmmap 和堆差异 JSON。
- `macos-object-native.memgraph`、原生 references / panel-trace / pools 日志；`macos-object-webcontent-40.memgraph` 是探索性快照，不能视为完整 JS 引用图。
- `macos-object{,-after}-go-idle-diff.log` 和 `macos-object-*-tests.log`。

启动 / 请求快照的完整命令见 [README](../README.md#macos-性能采样)。堆分类差异可使用：

```sh
python3 scripts/analyze-macos-heap.py \
  --before build/results/macos-object-webcontent-60-79965-heap.log \
  --after build/results/macos-object-webcontent-80-79965-heap.log \
  --output build/results/macos-object-webcontent-79965-batch-diff.json
python3 scripts/test-macos-heap-analysis.py
```

## 复现与证据

```sh
npm run build:macos
open -n "build/bin/Sidelet Spike.app" --args \
  -log-file "$PWD/build/results/macos-idle-launch.log"
# 从该日志读取 pid=，替换以下数字
npm run profile:macos -- --pid 12345 --duration 600 --interval 5
```

双 Stack 在启动参数中追加 `-two-stacks`。采样器生成 CSV、环境 JSON（含参数、哈希、初始进程）与统计 JSON；若进程退出、PID 被复用或计量失败，标记不完整。成员变化时，新加入进程的首段和已退出进程的最后一段 CPU 可能漏算，必须结合 `membershipChanged` 判断；本轮完成的场景成员均未改变。

原始文件保存在被 Git 忽略的 `build/results/`，不包含用户任务数据：

- `macos-idle-one-stack-20261004-162021.{csv,environment.json,summary.json}`。
- `macos-idle-two-stacks-20261004-170016.{csv,environment.json,summary.json}`。
- `macos-card-cycles-20261004-164655.{csv,environment.json,summary.json}`。
- `macos-card-cycles-events.jsonl` / `macos-card-cycles-check.json`。
- `macos-fullscreen-exit-check-20261004-165756.{csv,environment.json,summary.json}`。

下一轮仍需覆盖更长时间与完整 JavaScript 引用链、编辑备注和鼠标 Passive 查看稳定性、长时间 Hover 与画面丢帧，以及全局快捷键、多屏 / 拔插 / Dock 和真实硬件的 P0 交互。内存验收预算尚未冻结，完整进度见 [macOS 原型记录](phase-0-macos.md)。
