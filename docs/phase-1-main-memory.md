# macOS 主窗口内存排查 · 2026-10-06

本文为构建 8 的历史试验。当前构建 10 的逐窗口归因见 [使用后内存分项分析](phase-1-build10-window-memory.md)。

本轮没有发布新构建，也没有获得足够明确的新内存收益。日期格式器复用和移除任务列表裁切两项试验均已撤回；源码、构建目录应用和安装版保持构建 8。已有主窗口按需加载的启动收益仍见 [构建 8 记录](phase-1-memory-lifecycle.md)。

## 范围与口径

使用同一组 13 条合成任务与相同设置，普通模式启动，不启用 `-memory-dir` / `-interaction-test`，不打开快速添加或设置。依次记录后台启动、首次显示主窗口、隐藏主窗口，每段等待 15 秒后采样 30 秒，间隔 5 秒，共 7 点。表内为中位数；RSS 汇总可能重复计算共享页，另列 physical footprint。

原始 CSV、逐进程 JSONL、环境 / 二进制哈希、界面操作记录与试验应用保存在 `build/results/memory-main-20261006/`。`analyze.py` 检查采样完整、进程集合不变、同一实例 / 二进制、resource 与 jetsam coalition 双重归属，以及全部 SQLite 表与设置相同。主窗口进程根据首次创建时新增的唯一 WebContent 识别，期间不创建其他窗口。

## 两项撤回的试验

| 场景 | 构建 8 RSS / footprint | 日期格式器复用 | 去掉任务列表裁切 |
| --- | --- | --- | --- |
| 后台启动 | 241.8 / 90.5 MiB | 238.1 / 90.8 MiB | 237.4 / 90.6 MiB |
| 首次显示 | 328.1 / 158.5 MiB | 327.3 / 158.1 MiB | 326.6 / 157.0 MiB |
| 隐藏后 | 329.7 / 176.3 MiB | 328.9 / 175.5 MiB | 328.3 / 175.1 MiB |

隐藏后的主窗口 WebContent footprint 分别是 **74.83、74.83、74.94 MiB**。总量的约 0.8–1.2 MiB 差异没有对应的主窗口改善；不能据此宣称两项修改解决了内存问题，也不能用重启或诊断强制 GC 的低值替代正常使用后的结果。

日期格式试验复用了懒创建的 `Intl.DateTimeFormat`，保持本地时间显示，检查了午夜、闰日、夏令时和运行中时区切换；20 项前端测试与 Svelte 检查通过。ECMA-402 的 [Date 本地化格式定义](https://tc39.es/ecma402/2024/#sec-date.prototype.tolocalestring) 提供试验依据，但不会保证本机 WebKit 的节省量。没有实际内存收益，因此没有保留该修改。

裁切试验仅删除任务列表的 `overflow:hidden`，截图确认布局仍正常，但主窗口 footprint 未下降，同样撤回。没有进一步叠加未经对照的 CSS 或窗口销毁策略。

## 内存分类与边界

旧版与日期格式试验版的主窗口只读 `vmmap -summary` 都显示约 **50.7 MiB 的图形 owned-unmapped dirty**，是该进程 footprint 的主要部分。不能把这部分当作 JavaScript 对象，也不能把 vmmap 含共享系统库的 resident 总计与采样 RSS 相加。具体由哪些图形资源持有仍未定位，不能宣称已排除泄漏。

首轮旧版的 `old-hidden` 期间运行过 vmmap，这段已从干净对照中排除；后续用全新实例重新采集 `old-clean-visible` / `old-clean-hidden`，该实例不运行 vmmap。可见阶段 GPU 分配状态差异明显，不用其总 footprint 推断懒加载的收益或回归。

干净旧版隐藏后的 RSS / footprint 为 **342.03 / 173.65 MiB**，主窗口 WebContent footprint 为 **72.25 MiB**；当前构建 8 对应 **329.70 / 176.33 MiB**、**74.83 MiB**。本轮主窗口单独场景的总 footprint 差约 2.69 MiB，没有复现此前包含十次快速添加操作的约 15 MiB 差距。两轮工作流不同，因此不能宣称此前的差异已经修复，也不能排除其他工作流或缓存状态的关联；原约 15 MiB 的完整归因仍未完成。干净对照与逐进程识别依据见 `clean-comparison.json`。

本轮仅是短时定位与试验，不替代构建 7 的十分钟 / 多次工作流测试，也不改变 P0 人工验收状态。Quick Card 按需创建与缩小 Stack 空白视口两项仍未实施。

## 恢复与验证

两项试验仅在隔离资料目录运行，安装版没有被替换；源码与前端产物已恢复，原有 19 项前端测试和 Svelte 检查通过。`build/bin/Sidelet.app` 与 `/Applications/Sidelet.app` 均为构建 8，可执行文件 SHA-256 都为 `e90211bdcafe5f2ef1ebc1f98527e590c4f7a059d816df42a573a2021d6803aa`。试验二进制标记为构建 9，只保存在证据目录，不是发布产物。

所有测试实例已退出，正常安装版恢复为后台启动，日志为两条原任务、原保存的设置和已授权通知；全部 SQLite 表与设置文件和排查前备份精确相同，完整性 / 外键检查通过。恢复进程 PID 42491 的一次只读快照为 RSS **238.64 MiB**、footprint **88.96 MiB**、两个 WebContent；这是重启状态，不能用作使用后优化收益。相关验证保存在 `restored-validation.json` / `restored-processes.json`。

全窗口后续验收未复现主窗口隐藏视口试验的整体收益，构建 13 已撤回该试验，见 [全窗口验收与回退](phase-1-full-window-memory.md)。
