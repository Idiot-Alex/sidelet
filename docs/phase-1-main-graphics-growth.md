# 构建 14 主窗口重复流程与图形资源增长

2026-10-06，Apple M3 Max / macOS 26.6.2。本轮完成内存定位和复核工具，没有发布新构建，安装版仍为构建 14。

## 结论

**本轮复现的约 13 MiB 主窗口增长，主要落在图形资源，而不是已测到的 WebKit 分配区。** 独立分类实例中，主窗口 footprint 从 66.08 增至 78.95 MiB（+12.88），`owned unmapped (graphics)` dirty 从 39.3 增至 52.0 MiB（+12.7）；WebKit Malloc dirty 只增加约 0.2 MiB，VM / JIT 标签区域约增加 0.05 MiB。

普通模式两轮流程结束后，共用和独立弹窗版本的主窗口分别约 81.47 / 79.27 MiB，差距约 2.20 MiB。本轮没有复现上一轮完整工作流的约 21 MiB 主窗口差距，也没有获得新的优化收益。完整 JS 对象堆没有采集，具体是哪一批图层或绘制资源仍待定位，不能宣称已经排除泄漏或证明共用弹窗完全不影响内存。

下一处排查应聚焦任务页 / 设置页往返及主窗口重开时的图形资源保留。先读取 Web Inspector 的图层与 Memory timeline，确认哪些资源随页面切换增加，再做单项渲染试验；不重复已经撤回的主窗口缩小 / 销毁方案。WebKit 的 Memory timeline 能区分 JavaScript、图像、图层与其他页面内存，JS Allocations 则提供对象堆快照；这两项属于后续定位工具，本轮没有把它们写成已完成验收。参考 [WebKit 内存诊断说明](https://webkit.org/blog/6425/memory-debugging-with-web-inspector/)及[Timelines 说明](https://webkit.org/web-inspector/timelines-tab/)。

## 普通模式重复对照

两个隔离副本均由最终安装构建 14 复制，仅重新签名和分配独立 bundle / 显示名称。13 项合成任务、2 项固定任务与设置相同；用户正常实例保持运行。共享实例 `on2` PID 59758，独立弹窗实例 `off2` PID 63680。

每个实例连续执行两轮，每轮包含两次设置访问、十次空标题快速添加与 Escape 取消、隐藏主窗口；仅在两轮之间通过 LaunchServices 重开主窗口。每次打开和取消由应用的 `quick-add opened` / 成功恢复日志确认，全部展示会话编号为 1–20；任务库没有增加、删除或编辑。每轮全部界面操作结束后预热 10 秒，采样 30 秒、间隔 5 秒，共 7 点。两轮内部及跨轮 PID / 启动时间相同，每阶段同时校验 resource 与 jetsam coalition，WebContent 均为 3 个，总进程均为 6 个。

| 普通模式（中位数） | 独立窗口第 1 轮 | 独立窗口第 2 轮 | 共用窗口第 1 轮 | 共用窗口第 2 轮 |
| --- | ---: | ---: | ---: | ---: |
| 主窗口 footprint | 79.25 MiB | 79.27 MiB | 68.64 MiB | 81.47 MiB |
| 主窗口 RSS | 67.55 MiB | 67.70 MiB | 68.66 MiB | 69.25 MiB |
| 整组 footprint | 186.30 MiB | 184.86 MiB | 178.68 MiB | 191.60 MiB |
| 整组 RSS | 359.95 MiB | 363.19 MiB | 372.64 MiB | 375.64 MiB |

主窗口跨轮 footprint 差分别为 +0.02 / +12.83 MiB。两轮后才运行 `vmmap -summary`；共用实例的主窗口图形 dirty 为 52.0 MiB，独立实例为 52.4 MiB，WebKit Malloc dirty 分别为 17.3 / 16.3 MiB。普通模式没有 `-memory-dir`、交互 fixture、追踪、强制 GC 或窗口销毁。

这组流程聚焦主窗口，没有浮动卡片、整理、完成撤销或任务保存，因此不能用约 185–192 MiB 的整组读数替换[构建 14 完整使用流程](phase-1-shared-popup-memory.md)的结果。本轮仍只有每种模式一个冷启动实例、同一实例两次负载，不是足够规模的统计对照。

## 增长分项对照

为避免把分配诊断混入普通基准，另建同一构建的 `classify` 实例（PID 66020）。执行相同两轮操作，每轮预热 10 秒、采样 10 秒（3 点），随后才读取全部 WebContent 的 `vmmap -summary`。第二轮经过第一轮的 vmmap 读取，故整个实例仅用于分类，排除出上面的普通基准。

| 主窗口（同一 PID 66036） | 第 1 轮 | 第 2 轮 | 增量 |
| --- | ---: | ---: | ---: |
| 采样 footprint | 66.08 MiB | 78.95 MiB | +12.88 MiB |
| 图形 owned-unmapped dirty | 39.3 MiB | 52.0 MiB | +12.7 MiB |
| WebKit Malloc dirty | 16.2 MiB | 16.4 MiB | +0.2 MiB |
| JS VM / JIT 标签 dirty | 1.45 MiB | 1.50 MiB | +0.05 MiB |

图形项解释了该实例增量的主要部分。此处 RSS 不等于 footprint：图形资源的持有计费可以计入进程 footprint，而不表现为同等的映射驻留页增长。vmmap 的共享 resident、虚拟预留和图形 dirty 不相加；WebKit Malloc 包含引擎原生对象和分配，VM / JIT 标签也不是完整 JS 对象堆。

## 自动化干扰与归属限制

试测发现，后台辅助窗口的 `get_app_state` 有时触发应用的 Dock 重开入口，原生日志紧接着出现 `dock reopen control-only`，使尚在观察的快速添加切回主窗口。还发生过工具连接中断、辅助功能 / 坐标超时及首次窗口载入延迟。这些记录保留在 `off1` / `on1` / `off2/preflight`，不作为有效对照。

有效流程仅读取主窗口的辅助功能状态；快速添加显示后先等待自身打开日志，再通过 Computer Use 发送 Escape、等待成功恢复日志，期间不读取弹窗状态。两种模式使用相同脚本，每个有效实例都完整完成 20 次打开 / 取消。它验证的是受工具驱动的应用流程，不能当作真实全局快捷键或用户键鼠验收。

启动时主窗口和侧栏的 WebContent 进程创建次序存在竞争，不能用最先出现的 PID 一概认定主窗口。原始 vmmap 改为按实际 PID 命名；分析器在本轮只有 Main / Stack / Add 三种尺寸的场景中，通过唯一的大图形占用进程（dirty > 30 MiB）推断主窗口，并对三份完整分类、PID 身份与阶段成员做校验；没有直接的原生 page / PID 映射。若没有唯一候选，分析器报错，不能静默采用第一个进程。这项角色归属仍是基于场景的推断，后续更复杂窗口场景需要直接映射。

## 复核与恢复

后续执行了[主窗口固定视口试验](phase-1-main-scroll-trial.md)：独立容器内部滚动使图形占用更高，修改已经撤回，安装版仍为构建 14。分项流程中的设置往返、单独重开没有明显增长；原版在“第二轮操作 + 最终隐藏”后出现图形增加，这两步之间尚未单独分类，具体图层仍未确认。

新增 `scripts/analyze-macos-main-memory.py` 校验应用二进制、开关、隔离资料目录、两轮会话与设置次数、采样完整性、相同进程身份和双重 coalition、SQLite 完整性与外键、资料精确相同，以及 vmmap 分类。六项分析工具测试通过，覆盖 graphics dirty 与 shared resident 的区分、JS 虚拟预留、缺失分类、未知单位、启动顺序反转和角色归属歧义。

```sh
python3 scripts/test-macos-main-memory-analysis.py
python3 scripts/analyze-macos-main-memory.py --dir build/results/build14-main-repeat-20261006
```

证据目录 `build/results/build14-main-repeat-20261006/` 不进入 Git，保留计划、有效 / 失败流程、原始 CSV / 进程 JSONL、全部 vmmap、独立分类对照与 `main-comparison.json`。

全部隔离测试实例已经退出，正常 PID 43910 保持运行。正常资料全部表（含序列表）与此前备份精确一致，设置文件逐字节相同，原 2 项任务保持不变，SQLite 完整性 `ok`、外键无错误。安装应用 SHA-256 仍为 `850c510f05d306f19079bf0340f1351634f88cffc96bc9c4cd2b9c849505a3d6`，未替换应用或生成新发布包。主窗口缩小试验继续处于撤回状态，P0 的多屏 / Spaces / 睡眠等验收状态不变。
