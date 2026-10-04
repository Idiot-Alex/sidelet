# Phase 0 · Edge Window Spike

状态：**Development Candidate，Windows 实测待完成**。

本文件保留 Windows 验收清单。按用户当前的 M3 Mac 开发需求，优先推进 macOS 原型，见 [macOS 记录](phase-0-macos.md)；Revision 4 原文未修改。

本阶段验证 Revision 4 第 3.1 节的 Overlay 架构。使用 8 条内存假数据；正式业务开发从原型通过验收后开始。

## 实现方案

- Wails / Runtime：`v3.0.0-beta.27`；Go：`1.27.1`；Svelte：`5.57.1`。
- 隐藏的诊断面板、1～2 个 Stack 窗口，以及复用的 Quick Card 窗口。
- 默认 1 个 Stack 展示 8 条任务；`-two-stacks` 使每个 Stack 展示 4 条互不重复的任务。
- Stack 窗口覆盖当前 WorkArea 内侧的一条窄区域。前端只在布局、状态和尺寸变化时上报 DOM 命中矩形；平台层转换为物理坐标，用 `CreateRectRgn / CombineRgn / SetWindowRgn` 设置区域并排除间隙。
- Passive 使用 `WS_EX_NOACTIVATE` 和 `WM_MOUSEACTIVATE → MA_NOACTIVATE`；显式键盘操作或编辑暂时允许激活，并记录之前的前台窗口。
- 显示器、DPI、WorkArea 与前台窗口变化使用原生事件。全屏判断采用前台窗口覆盖显示器 Bounds 的启发式。
- Snooze / 800ms 完成反馈 / Undo 使用下一截止时刻的一次性定时器。
- Stack 与 Quick Card 上报鼠标进入/离开，Go 串行控制器维护同一查看会话。离开两者 500ms 后使用一次性定时器关闭；重新进入或打开新的卡片使旧的关闭消息失效。KeyboardActive / Editing 期间暂停自动关闭。
- 打开的卡片锁定来源 Stack 中的任务展开；第二个 Stack 不参与该卡片的悬停计时。Quick Card 的键盘任务范围也随来源 Stack 切换。
- 键盘选择超出直接显示区时，暂时用选中任务替换末行；`+N` 收纳其余任务，不修改内存假数据的顺序。
- Hover 采样只在展开后约 200ms 内使用 requestAnimationFrame。

当前方案优先验证动态 Window Region。若 P0 命中或焦点失败，记录失败证据，再比较其他实现方式。

## 本地验证记录

开发环境：macOS arm64，Node `24.11.0`，Go `1.27.1`。本地结果只能证明构建、逻辑及浏览器内交互。

| 检查 | 状态 |
|---|---|
| Svelte / TypeScript 检查 | 通过，0 errors / 0 warnings |
| 前端生产构建 | 通过 |
| 几何与浏览器假数据逻辑测试 | 10 项通过 |
| Go 假数据及跨窗口查看会话测试，含 race 检查 | 6 项通过 |
| Windows 平台代码 vet | 通过 |
| Windows amd64 / arm64 交叉编译 | 两种架构均通过，已生成 GUI 可执行文件 |
| 浏览器被动输入与 Hover 保持输入框焦点 | 通过 |
| 浏览器完成 / Undo | 通过 |
| 浏览器编辑 / Snooze | 通过 |
| 浏览器键盘选择 / Esc | 通过 |
| 浏览器短工作区的键盘选择 / 打开聚合中的任务 | 通过：180px 工作区内第 3 条任务显示在第 2 行，Enter 打开对应卡片 |
| 浏览器编辑取消返回键盘模式 / Esc 清理展开 / 恢复输入焦点 | 通过 |
| 浏览器标签 → Quick Card / 离开后 500ms 关闭 | 通过 |
| 浏览器编辑期间离开卡片 | 通过：保持编辑，不执行悬停关闭 |
| 浏览器小工作区 +N 入口 | 通过 |
| 浏览器短工作区卡片边界 / 内部滚动 | 通过：180px 工作区内卡片限制为 160px |
| 浏览器左右切换 / 安静模式恢复 | 通过 |
| 浏览器键盘模式 → 安静模式 → 快捷键 / 恢复 | 通过：隐藏期间保持 Passive，键盘入口禁用，恢复后 8 条任务可见 |
| Windows 原生命中、焦点、DPI、显示器、全屏行为 | 待实测 |
| Windows 10 分钟空闲性能与重复 Hover 稳定性 | 待实测 |

浏览器帧回调频率不能作为 Windows 绘制性能验收结果；PowerShell 脚本也需要在目标机器运行验证。

## 测试环境记录

每轮单独保存目录，记录机器、版本和配置。启动日志包含平台、Go / Wails 精确版本、显示器 ID、coordinateScale、backingScale 和 WorkArea。Windows 的 coordinateScale 为 DPI 比例，WorkArea 为物理像素；采样脚本生成环境 JSON 与进程树 CSV。

| 字段 | Windows 实测填写 |
|---|---|
| Windows 版本 / 构建号 | 待填写 |
| WebView2 Runtime 版本 | 待填写（采样 CSV 含实际进程 FileVersion） |
| CPU 型号 / 架构 / 逻辑核数 | 待填写 |
| 显示器数量 / 分辨率 | 待填写 |
| 每块显示器 DPI / Scale | 待填写（以原型日志为准） |
| 主窗口状态 / Stack 数量 / 标签高度 | 待填写 |
| Binary SHA256 | 采样环境 JSON |
| 测试起止时间 / 操作者 | 待填写 |

## P0 · 命中与焦点

下层至少使用两个不同进程的应用，建议 Chrome 和编辑器。浏览器预览的 textarea 不替代原生测试。

| 场景 | 必须观察到的行为 | 结果 |
|---|---|---|
| 折叠标签 Hover | 约 150ms 后展开 | 待测试 |
| 标签间隙点击 | 下层应用收到点击 | 待测试 |
| 标签周围透明区域点击 | 下层应用收到点击 | 待测试 |
| 透明间隙与周围滚轮 | 下层应用滚动 | 待测试 |
| 展开区域点击 / checkbox | 打开 Quick Card / 完成对应任务 | 待测试 |
| 在编辑器输入 service 并 Hover | S、E、Space、方向键仍属于编辑器 | 待测试 |
| Todo → Quick Card | 移动过程不意外收起 | 待测试 |
| Quick Card 查看 | 前台应用继续保持输入焦点 | 待测试 |
| Ctrl+Alt+T | 显式进入键盘模式 | 待测试 |
| Space / S / E / ↑ / ↓ | 只在键盘模式中作为任务操作 | 待测试 |
| Esc / 点击外部 | 退出键盘模式，外部点击尊重用户选择 | 待测试 |
| 快速编辑 | 输入可用；取消/保存按上下文返回 | 待测试 |
| 编辑器 → 键盘模式 → Esc | 尽可能恢复原前台窗口 | 待测试 |
| 标签收起 | 命中区域随之缩小，间隙恢复穿透 | 待测试 |
| 单个 Stack 8 条任务 | 确认只有一个 Stack WebView | 待测试 |

若出现激活拒绝、全局快捷键冲突、区域安装失败，保留日志与触发步骤，不视为通过。

## P1 · 几何与系统行为

1. 在左右边缘分别测试 offset=0、0.35、1；任务完成后检查补位与边界。
2. 测试 100%、125%、150%、175%、200% DPI，以及混合 DPI 双屏。
3. 将标签高度调到 64px，在小分辨率/高 DPI 工作区检查动态 `+N`。聚合入口占一行高度预算。
4. 通过编辑备注生成长内容，在屏幕顶部和底部打开 Quick Card，确认中间内容滚动，标题和操作区可见。
5. 改变任务栏位置与大小，检查窗口重新校正。
6. 使用 `-two-stacks` 断开目标显示器；检查迁移到主屏空闲侧，重连后保持迁移位置。
7. 分别测试视频、演示、游戏、远程桌面全屏；记录启发式的误判/漏判。安静模式恢复后检查原布局。
8. 重复打开/收起、编辑/取消、完成/撤销，检查 Quick Card 复用与 NoActivate 恢复。

WorkArea 太小到连一条任务与聚合入口也无法容纳时，原型降级为仅显示 `+N`，并将该行高度限制为剩余工作区高度。几何测试覆盖 0～60px 工作区；真实设备上的可操作性仍需记录，极小工作区不视为已完成原生验收。

## P2 · 性能与稳定性

使用 README 的 Start-Process 与 `scripts/profile-windows.ps1`，逐个记录下列场景。CPU 百分比按逻辑核数归一化；首条采样没有 CPU delta。Working Set 求和可能重复计算共享页，结合 Private Bytes 判断。

| 场景 | Root Working Set | WebView2 Working Set | Total Private Bytes | Idle CPU | Hover CPU 峰值 | 画面 FPS / 首次展开延迟 |
|---|---|---|---|---|---|---|
| 主窗口隐藏，1 Stack / 8 条，Collapsed | 待测试 | 待测试 | 待测试 | 待测试 | — | — |
| Hover / Expanded | 待测试 | 待测试 | 待测试 | — | 待测试 | 待测试 |
| Quick Card 打开 | 待测试 | 待测试 | 待测试 | 待测试 | — | 待测试 |
| 空闲 10 分钟 | 待测试 | 待测试 | 待测试 | 待测试 | — | — |
| 双屏 / 2 Stack | 待测试 | 待测试 | 待测试 | 待测试 | 待测试 | 待测试 |
| 反复 Hover / 收起与 Quick Card 复用 | 待测试 | 待测试 | 待测试 | 待测试 | 待测试 | 待测试 |

Hover 日志中的 delayMs 包含 150ms 的意图等待；fps 为短时帧回调频率。最终动画验收结合 WebView2 DevTools Performance trace 和肉眼观察，记录丢帧情况。

采样脚本默认每 5 秒采样，空闲场景 600 秒；Hover 峰值可用 `-IntervalSeconds 1` 采样，但更短暂的峰值需要性能分析工具。

## 冻结条件

Windows P0 命中与焦点通过，P1 边界与系统行为满足规则，P2 建立可信 baseline 并记录稳定性结果后，填写结论与选择的窗口方案，升级为 Development Baseline。

当前不填写虚构的 Windows 性能数字，产品规格保持不变。

## 接口依据

- [Wails v3 Window Options](https://v3.wails.io/features/windows/options/)
- [Wails beta.27 源码](https://github.com/wailsapp/wails/tree/v3.0.0-beta.27/v3/pkg/application)
- [Microsoft SetWindowRgn](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowrgn)：区域坐标相对于窗口；成功后区域对象由系统接管。
- [Microsoft SetWindowSubclass](https://learn.microsoft.com/en-us/windows/win32/api/commctrl/nf-commctrl-setwindowsubclass)：子类安装与窗口位于同一线程。本原型将平台操作交由 Wails UI 线程执行。
