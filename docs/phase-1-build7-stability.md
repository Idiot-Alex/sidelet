# 构建 7 · macOS 稳定性复测

2026-10-06，针对 `/Applications/Sidelet.app` 的 0.1.0 构建 7，覆盖 Quick Add 与导出增量。沿用 [性能口径](phase-0-macos-performance.md)，[构建 2 记录](phase-1-macos-stability.md)保留为历史结果。本轮功能工作流通过，完成普通模式和诊断模式的完整恢复观察；初始空闲 CPU 略超首轮目标，内存保留的完整归因仍未完成，不将性能或 P0 整体升级为通过。

## 场景与隔离

证据目录为 `build/results/stability-build7-20261006/`，不进入 Git。正常实例退出后，在独立资料目录创建两条合成固定任务 `Stability A / B`，不使用日常任务做修改实验。正常资料库及设置在开始时保存比较基准。

所有阶段使用同一安装版二进制：`0b9b7a334b0131e42228689882ae92bb7c2741d500c3317c432f77553eae5334`。机器为 Apple M3 Max / arm64，14 个逻辑核、36 GiB、macOS 26.6.2，单块 Retina 屏幕、一个 Stack。

1. 普通模式：不带 `-memory-dir`、`-interaction-test` 或追踪参数。主窗口隐藏，预热 30 秒后每 5 秒采样 600 秒；随后在同一 PID 完成 100 次快速添加 / 取消，隐藏主窗口，再预热 30 秒并完整采样 600 秒恢复。
2. 诊断模式：退出前一实例，再启用内存快照和项目测试入口。实际完成 100 次快速添加、25 次设置往返及任务工作流，分批取得 Go GC 后堆、原生弱引用计数与前端生命周期计数。
3. 诊断操作结束：隐藏主窗口并退出夹具，保持同一 PID 观察至少 600 秒，再取最终快照。该实例的轮询、快照 GC、追踪与界面观察会改变时序，其 CPU 不替代普通模式基线。

CPU 包含主进程及同 resource / jetsam coalition 的 WebKit 进程；100% 为占满一个核，归一化数字再除以 14。RSS 和 physical footprint 分别报告，共享页可能重复计入 RSS。以本机约 300ms 忙循环和 `getrusage` 交叉核对 CPU 单位，Mach timebase 转换后的比值为 1.000019；原始 tick 直接当纳秒会低估约 41.7 倍。

## 普通模式的完整采样

| 指标 | 初始空闲 | 100 次开关后的恢复 |
|---|---:|---:|
| 时长 / 样本数 | 600.025 秒 / 121 | 600.026 秒 / 121 |
| CPU 加权平均，单核 | 0.502939% | 0.477108% |
| CPU 加权平均，14 核归一化 | 0.035924% | 0.034079% |
| CPU 中位数 / 区间峰值，单核 | 0.037151% / 3.782479% | 0.446145% / 3.259283% |
| 总 RSS 首值 → 末值 | 313.250 → 314.781 MiB | 426.609 → 426.797 MiB |
| 总 RSS 中位数 | 313.578 MiB | 426.484 MiB |
| 总 footprint 中位数 | 129.786 MiB | 196.662 MiB |
| 归属进程数 | 6 | 7 |
| 每段内部成员变更 | 无 | 无 |

初始空闲平均 CPU 略高于规格第一轮 0.5% 目标，不能将本轮该项写为通过。后半段出现升高，峰值完整保留。恢复平均值低于目标，但仍存在超过目标的采样区间，两段不互相抵消。

恢复时 RSS 已处于平台期，却比初始末值保留约 112 MiB；首次快速添加增加一个缓存 WebView 是预期行为，不能据此解释全部保留内存，也不能凭平台期宣称零泄漏。两段结束之后才取三秒只读调用栈，未污染基线；以事件等待为主，初始末段也有少量 WebKit 显示刷新，不足以定位 CPU 根因。

按两段末值拆分，新增的 Quick Add WebContent 进程自身占 RSS **63.78 MiB**、footprint **27.59 MiB**；主进程 RSS 增加 **21.12 MiB**，其他既有进程合计增加约 **27.11 MiB**。最终主进程 RSS 为 **135.05 MiB**，其余属于 WebKit 辅助进程。因此总量不能只看活动监视器中的 Sidelet 主进程一行，全部增量也不能都归给新增窗口。逐进程比较保存在 `normal-retained-processes.json`。

## 内存占用的原因

普通模式恢复末值的组成如下；这是同一时刻的 RSS，相加可能重复计入共享页，不能当作全部私有内存。

| 进程类别 | RSS |
|---|---:|
| Sidelet 主进程 | 135.05 MiB |
| 四个 WebContent | 221.89 MiB |
| WebKit GPU | 52.78 MiB |
| WebKit Networking | 17.08 MiB |
| 合计 | 426.80 MiB |

代码和快照共同确认的常驻开销：

1. `cmd/spike/main.go` 启动时创建任务主窗口、每个 Stack 和 Quick Card。即使主窗口 / Quick Card 不可见，也已加载网页；冷启动实际有三个 WebView / WebContent。
2. `cmd/spike/quick_add.go` 首次创建第四个窗口，取消和保存后调用 Hide，只清空录入状态，不销毁 WebView。主窗口关闭钩子同样取消真正关闭并执行 Hide。当前没有闲置窗口回收策略。
3. 锁定的 Wails beta.27 为每个窗口创建 `WKWebViewConfiguration` 和 `WKWebView`，Hide 在 macOS 上只执行 `orderOut:`，不会卸载页面。WebKit 使用多进程架构，内容进程以外还有共享网络 / 渲染服务，见 [WebKit 架构文档](https://docs.webkit.org/Deep%20Dive/Architecture/WebKit2.html)。因此一个小任务窗口也带有网页运行环境的固定成本，不能只按任务字符串大小估算内存。

Go GC 后存活堆约 2.5 MiB，goroutine 与已统计对象没有累计增加；前端 JS / CSS 文件合计不足 200 KiB。这些证据不支持把几百 MiB 全部归因于任务数据或脚本文件体积。Go 堆不包含 AppKit、WebContent 的完整 JavaScript 堆及 GPU 资源，也不能据此排除这些部分的泄漏。

另一个需要验证的渲染因素是：Stack 的原生窗口宽 312、覆盖整个工作区高度，空白部分虽然完全透明，WebView 的视口仍覆盖这个矩形。它可能增加图形表面和渲染缓存开销；这属于根据尺寸的推断，尚未取得收窄视口前后的对照，不能把全部 GPU 内存归给透明区域。

优化顺序应先减少常驻 WebView：优先让任务主窗口首次使用时创建，并评估无草稿、无保存中的快速添加闲置回收；Quick Card 按需创建需同时验证首次悬停延迟，不能只追求低内存而牺牲响应。随后做紧凑 Stack 视口的对照。回收必须处理草稿、前端订阅、原生解绑和焦点恢复，不能以丢弃用户输入换低内存。具体节省量仍需同一场景实测，不预先承诺低于某个固定上限。

### 系统区域分类

完整恢复和进程采样结束后，才对七个已确认进程分别取 `vmmap -summary -w`，未影响恢复统计。该分类对应诊断实例，不能直接替换上述普通模式数字：

- 主进程 footprint 47.3 MiB。DefaultMallocZone 的 resident 21.5 MiB、dirty 17.0 MiB、报告 allocated 10.3 MiB，dirty / swap 碎片约 6.68 MiB；VM_ALLOCATE resident 15.9 MiB。Go 存活堆不是主进程的全部 Go / 原生运行开销。
- Quick Add WebContent footprint 23.6 MiB。WebKit Malloc zone resident 32.7 MiB、dirty 11.7 MiB、报告 allocated 8.33 MiB。它提示分配池保留和碎片，但不能把 resident 与 allocated 的全部差额认定为可立即释放的缓存，更不能以此排除内部对象泄漏。
- GPU footprint 20.5 MiB，峰值 236.9 MiB；IOSurface 虚拟映射 478.7 MiB、当时 resident 为零，图形 owned-unmapped 的 dirty 约 4.61 MiB。不能将巨大的 IOSurface 虚拟值写成同样大的真实内存。

WebContent 的 Gigacage 等保留数十 GiB 地址空间，但这些 reserved 行 resident 为零。`vmmap` 各区域的 resident 还包含系统共享库映射，口径不能与进程采样器 RSS 简单等同或再相加。[WebKit 内存检查文档](https://docs.webkit.org/Infrastructure/MemoryInspection.html)区分 dirty / clean / reclaimable，细到内部对象类别的堆拆分还依赖 WebKit 构建配置；本轮使用系统框架，仅取得区域 / zone 分类和有限的生命周期计数，没有完整 JavaScript 引用链。

普通模式 100 次取消逐次确认返回任务页，每五次输入可识别时间前缀，取消后输入清空；数据库所有表前后精确相同。第 83 次出现 Computer Use 读取界面时触发 Dock reopen 的中断，原始失败单独保存在 `retries.jsonl`，成功重做后才计数。它不证明真实全局快捷键有问题或没有问题。

## 诊断模式的重复操作与对象计数

100 次 Quick Add 包含十次 Enter 保存、90 次 Esc 取消；每十次保存一条“明天 15:30”的未固定、不提醒任务。另完成 25 次设置页往返、五次完成 / 五秒内撤销、五次延后 30 分钟 / 现在恢复，以及五次固定临时任务创建 / 完成 / 撤销期后自动清理。

| 指标 | 冷启动 | 首次快速添加 | 20 次 + 5 设置 | 60 次 + 15 设置 | 100 次 + 25 设置 | 全部任务操作后 |
|---|---:|---:|---:|---:|---:|---:|
| Go GC 后 HeapAlloc（KiB） | 2406.18 | 2438.75 | 2612.94 | 2541.37 | 2610.45 | 2598.43 |
| goroutine | 15 | 15 | 15 | 15 | 15 | 15 |
| 附着 WebView | 3 | 4 | 4 | 4 | 4 | 4 |
| AppKit 窗口计数 | 7 | 8 | 8 | 8 | 8 | 8 |
| 窗口 observer / mouse monitor | 8 / 2 | 12 / 2 | 12 / 2 | 12 / 2 | 12 / 2 | 12 / 2 |
| 接收面板存活 / 应用持有 | 2 / 2 | 2 / 2 | 2 / 2 | 2 / 2 | 2 / 2 | 2 / 2 |

首次 Quick Add 创建之后，窗口、附着 WebView、已统计监听和 goroutine 不再随循环累计增加。任务隐藏 / 恢复、固定 / 清理使接收面板累计创建数由 2 增至 22，结束时存活和应用持有均回到 2。Quick Card 累计挂载六次，结束时只存活一份。

每个视图始终只有一份 App、13 个桥接订阅、三个 document/window 监听和一个 ResizeObserver。Quick Add 当前 DOM 始终为 32，Stack 为 18；主窗口从 126 增至 238，包含新增的十条普通任务和一条提醒任务，并非相同任务规模的对象增长。计数只覆盖已插桩对象及当前 DOM，不覆盖脱离文档的节点、完整 JavaScript 引用链和全部定时器。

## 提醒、导出与重启

通过 Quick Add 创建时间前缀标题 `今天02:25 =1+1 合成提醒`，在任务页编辑中文多行备注并显式勾选“提醒我”。02:25 系统通知接口接受请求，SQLite 持久化回执，主窗口显示“提醒已交给系统”。本轮没有打开通知中心，未将接口成功写成横幅可见性验收；通知权限保持原来的“已允许”。

原生保存窗口取消一次，再分别导出 JSON 和 CSV 到证据目录。两种文件均含 13 条相同任务 ID；JSON 的全部任务与布局字段逐项等于 SQLite，CSV 的每一列按格式转换后与 JSON 一致，覆盖中文、多行备注、提醒回执与布局。CSV 的公式标题带保护前缀，使用 CRLF，包括单元格内部换行，比对时规范化换行；第一次直接字符串比较失败保存在 `export-verification.json`，没有将其误判为内容丢失。复核脚本 `verify-exports.py` 也保存在证据目录。

五条临时任务已全部自动移除，SQLite 完整性为 `ok`、无外键错误。完整恢复后才退出诊断实例，用同一安装版、不带诊断参数打开同一测试库：全部表、设置文件逐字节、提醒回执均与重启前相同；实际 UI 显示 13 条任务和已提交系统的提醒状态，启动日志无新的通知提交。独立的 `restart-before.json`、`restart-after.json` 与验证结果保存在证据目录，没有以重启后的低内存冒充恢复。

## 恢复期间的中断

第一段诊断恢复途中，日志记录到多次 Hover，并在 02:33:47 出现 Dock reopen、主窗口变为可见；之后 CPU 与 RSS 上升。没有将这段当作连续静置，也没有推断操作来自用户、观察工具或哪一种硬件输入。原始采样全部保留，`phases.json` / 分析结果记录中断区间，重新隐藏窗口并从 02:35:13 开始完整恢复计时。恢复后的快照仍为四个 WebView、八个窗口、15 个 goroutine，测试数据库全部表未变化。

完整采样随后覆盖关闭后的 **607.017 秒**；第 570–600 秒六点的 RSS 中位数 **512.133 MiB**、footprint **204.405 MiB**。恢复第 30–600 秒 RSS **514.125 → 512.141 MiB**，七个成员未更换；单核 CPU 加权平均 **1.265079%**、中位数 **0.779629%**、区间峰值 **4.324104%**。该实例开启诊断与追踪，不作为普通空闲预算，也没有将高出的部分全部归因于诊断。

最终 Go GC 后 HeapAlloc **2604.43 KiB**，goroutine **15**，四个 WebView、八个窗口、12 / 2 个 observer / mouse monitor、两块持有 / 存活接收面板，以及每视图的订阅和监听数均未增加。后半段发生其他应用前台切换，Sidelet 主窗口仍隐藏、无新增 Hover / Dock reopen；本轮是 Sidelet 静置，不声称整个桌面全程无活动。复测含未计划的中断操作，最终内存不能只归因于计划中的 100 次循环；原始 360 点完整保留。

## 收尾

测试实例和夹具全部退出，恢复 `/Applications/Sidelet.app` 的日常资料目录与“全部 2”筛选。正常数据库所有表精确相同、设置逐字节相同、完整性为 `ok`；两个快捷键均注册成功，登录启动保持关闭，通知授权仍为允许。应用二进制 SHA-256 未变，本轮没有重打包或修改 DMG。最终验证在 `final-validation.json`，分析结果为 `analysis.json`。

## 回归与复核

前端 19 项测试、Go race / vet 已通过。受执行沙箱限制，首次原生 AppKit 测试未完成；退出本轮测试进程后，在允许 GUI 的执行环境单独重跑，33 项原生路由、16 项透明背景、九项 Dock 断言通过。

新增真实 SQLite 集成测试覆盖时间解析 → 显式固定 / 开启提醒 → Snooze → 完成 / Undo → 重启 → 延后提醒回执 → JSON / CSV 导出 → 再次重启不重复发送。系统通知使用可控替身，此测试与上面的实机系统接口证据分别说明。集成包 race 检查通过。

分析器扩展为支持首次才创建的第四个 Quick Add 视图、同一普通模式 PID 的完整恢复，以及工作流成功次数。七项离线检查通过，历史构建 2 的原始证据仍可复核。

```sh
python3 scripts/analyze-macos-stability.py --dir build/results/stability-build7-20261006
python3 scripts/test-macos-stability-analysis.py
bash scripts/go.sh test -race ./internal/integration
```

分析结果的 `evidenceComplete` 只表示计划的证据齐全，不表示零泄漏或 P0 通过。真实 Control + Shift + Space、中文输入法候选确认、多屏 / Spaces / 全屏、全天运行和完整保留内存归因仍待验证。
