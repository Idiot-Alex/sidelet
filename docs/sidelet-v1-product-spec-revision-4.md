# Sidelet V1 Product Spec

## 1. 产品定位

Sidelet 是一款常驻桌面的轻量 Todo 工具。

它不是传统 Todo 软件简单增加一个「Always On Top」窗口，而是把每一条 Todo 看作一个可以存在于桌面上的轻量对象。

核心目标：

> 让用户无需打开 Todo 软件，也能持续感知重要任务，并通过极低干扰的方式完成查看、处理和编辑。

核心体验：

**Glance → Hover → Act**

- Glance：一眼知道还有任务。
- Hover：鼠标经过即可知道任务是什么。
- Act：无需进入主程序即可完成、稍后、编辑。

核心原则：

> Edge Todo 默认只表达“任务存在”，Hover 才表达“任务是什么”，点击之后才提供“如何处理”。

---

## 2. 产品设计原则

### 2.1 桌面空间比功能更贵

任何常驻桌面的元素都必须有存在价值。

默认设计应做到：

- 不遮挡工作内容。
- 不抢焦点。
- 不打断输入。
- 不持续吸引注意力。
- 不使用夸张动画。
- 不使用高饱和度颜色。
- 不主动制造焦虑。

### 2.2 渐进式信息展示

信息按照用户意图逐级出现。

```text
Level 0：存在感
    ↓ Hover
Level 1：任务信息
    ↓ Click
Level 2：快速操作
    ↓ Edit
Level 3：完整管理
```

复杂度越高，进入层级越深。

---

## 3. V1 技术栈

```text
Go
+
Wails
+
Svelte
+
TypeScript
+
SQLite
+
少量平台 Native API
```

Windows 为 V1 首发平台。

未来支持：

```text
macOS
```

Native API 必须封装在平台层，不允许散落在业务代码中。

### 3.1 开发前技术 Spike

正式进入 Todo CRUD 和 SQLite 业务开发前，必须先完成一个最小 `Edge Window Spike` 原型。

Spike 不是普通性能 Demo，而是：

> **V1 Overlay 架构可行性验证。**

只使用 8 条假数据，不连接正式数据库。

必须记录并锁定测试环境：

```text
Go version
Wails version
Windows version
WebView2 Runtime version
CPU architecture
DPI scaling
显示器数量
显示器分辨率
```

其中 Wails 版本必须记录项目实际使用的精确版本，不能只写“最新版”。

#### 3.1.1 P0：局部鼠标命中

一个 EdgeStack Window 中可以同时存在多条 Todo。

例如：

```text
EdgeStack Window
│
├── Todo A           ← 可 Hover / Click
├── 透明间隙         ← 鼠标事件穿透到下层程序
├── Todo B           ← 可 Hover / Click
├── 透明间隙         ← 鼠标事件穿透
├── Todo C Expanded  ← 可 Hover / Click
└── 周围透明区域     ← 鼠标事件穿透
```

必须验证：

| 场景 | 验收要求 |
|---|---|
| 鼠标进入折叠 Todo 标签 | 能触发 Hover |
| Todo 之间透明间隙 | 下层应用正常收到鼠标事件 |
| 点击透明区域 | 点击落到下层 Chrome / IDEA 等程序 |
| 透明区域滚轮 | 下层应用正常滚动 |
| Todo 展开 | 展开区域可 Hover / Click |
| 点击 checkbox | 可以完成 Todo |
| Hover Todo | 当前工作应用不丢键盘焦点 |
| Todo → Quick Card | 移动过程不闪退、不意外收起 |
| Quick Card 查看状态 | 不主动抢输入焦点 |
| Quick Card 编辑输入框 | 此时才允许获取输入焦点 |
| Todo 收起 | 透明区域重新恢复穿透 |
| 8 条 Todo | 仍然只使用 1 个 EdgeStack Window |

不能把实现简单等同于：

```text
ClickThrough = true / false
```

因为 V1 真正需要的是：

> **窗口内部局部 Hit Region。**

Spike 中优先比较：

```text
动态 Window Region
Native Hit-Test
窗口级 ClickThrough 切换
其他可行方案
```

可以重点验证 Windows 原生能力，例如：

```text
SetWindowRgn
CombineRgn
WM_NCHITTEST
```

但 Product Spec 不提前锁死实现方式，最终以 Spike 实测结果为准。

#### 3.1.2 P0：焦点与键盘规则

必须验证：

```text
Hover Edge Todo
→ 不抢输入焦点

Hover Expanded Todo
→ 不抢输入焦点

Quick Card 查看
→ 不主动抢输入焦点

Passive Mode
→ Space / S / E / ↑ / ↓ 不被 Sidelet 消费

主动进入 Keyboard Interaction Mode
→ 才允许消费 Todo 操作按键

真正进入输入/编辑
→ 才允许获得文本输入焦点

Esc 退出 Keyboard Interaction Mode
→ 恢复 Passive Mode
→ 尽可能恢复之前的前台应用
```

#### 3.1.3 P1：窗口行为与边界

必须验证：

```text
TopMost
NoActivate
Edge Snap
Hover 展开/收起
Quick Card 打开/关闭
全屏应用隐藏
DPI 变化
多显示器变化
```

同时必须验证：

```text
Stack 高度发生变化后仍完整可见
Expanded Todo 不越出 WorkArea
Quick Card 在屏幕顶部/底部附近打开时仍完整可见
任务栏改变 WorkArea 后位置重新校正
100% / 125% / 150% / 200% DPI 下边界计算正确

当 Stack 高度超过 WorkArea
→ 自动减少直接显示数量并进入 +N

当 Quick Card 自然高度超过 WorkArea
→ 限制高度并在内容区内部滚动
```

#### 3.1.4 P2：性能与稳定性

必须验证：

```text
1 个 EdgeStack
8 条 Todo

2 个 EdgeStack
各自包含多条 Todo

Collapsed
Expanded
Quick Card

空闲运行 10 分钟
长时间重复 Hover / 收起
```

并记录：

```text
主进程 Working Set
WebView2 相关进程 Working Set
总 Private Bytes
Idle CPU
Hover CPU 峰值
动画 FPS
首次展开延迟
```

#### 3.1.5 Spike 结论规则

如果：

```text
一个 WebView Window
+
局部鼠标命中
+
透明区域穿透
+
Hover 不抢焦点
```

无法稳定实现，则必须：

> 在正式 Todo / SQLite 业务开发前调整 Overlay 架构。

不能在业务层完成后再补救。

Spike 通过后，文档状态才允许从：

```text
Development Candidate
```

升级为：

```text
Development Baseline
```

---

## 4. 产品显示模式

产品总体设计支持四种显示形态，但 **V1 实际只交付 Edge Todo**。其余形态属于后续版本规划。

### 4.1 Edge Todo — V1

核心特色功能。

每一条 Todo 可以独立吸附在屏幕左侧或右侧。

收起状态：

```text
                    ┣━━
                    ┣━━
                    ┣━━
                    ┣━━
```

鼠标经过：

```text
      □ 准备周会材料   16:00 ━━━┫
```

不是：

```text
工作
个人
项目
```

边缘标签对应的必须是：

> 一条具体 Todo。

例如：

```text
完成需求文档
回复重要邮件
准备周会材料
阅读 20 分钟
预约体检
整理报表
```

### 4.2 Floating Ball — V1.1

用于不希望屏幕边缘存在多个标签的用户。

默认：

```text
   ✓
   5
```

Hover：

```text
┌──────────────────┐
│ 今日还有 5 项     │
│                  │
│ □ 修复接口       │
│ □ 回复客户       │
│ □ 发布测试       │
└──────────────────┘
```

点击进入快速 Todo 面板。

### 4.3 Floating Cards — V1.1

Todo 可以作为半透明卡片独立存在于桌面。

例如：

```text
┌──────────────────┐
│ 工作             │
│                  │
│ □ 修复接口       │
│ □ 发布版本       │
└──────────────────┘
```

适合用户主动选择少量任务长期展示。

### 4.4 Main Window — V1

完整 Todo 管理界面。

V1 只保留：

```text
Inbox
Today
Pinned
Completed
Settings
```

暂时不建设复杂：

```text
Workspace
Team
Kanban
大型项目系统
复杂过滤器
```

---

## 5. Todo 独立显示模型

显示模式不是整个 App 的全局单选模式。

每条 Todo 可以独立决定自己的桌面表现。

例如：

```text
Todo A → Edge Todo
Todo B → Floating Card
Todo C → 普通 Todo
Todo D → Focus Todo
```

业务层不再使用 `Pinned bool`。

“固定到桌面”只是用户界面的表达，内部统一由桌面呈现模型决定。

### 5.1 DesktopDisplayMode

```go
type DesktopDisplayMode string

const (
    DisplayNone  DesktopDisplayMode = "NONE"
    DisplayEdge  DesktopDisplayMode = "EDGE"
    DisplayCard  DesktopDisplayMode = "CARD"
    DisplayFocus DesktopDisplayMode = "FOCUS"
)
```

规则：

```text
DisplayMode = NONE
→ 不在桌面 Overlay 中展示

DisplayMode = EDGE
→ 进入 Edge Stack

DisplayMode = CARD
→ 作为独立 Floating Card 展示

DisplayMode = FOCUS
→ 作为当前 Focus Todo 展示
```

Floating Ball 属于应用级入口，不属于单条 Todo。

### 5.2 Todo 与桌面呈现解耦

建议将任务数据与桌面呈现分开。

```go
type Todo struct {
    ID           int64
    Title        string
    Description  string

    Completed    bool
    CompletedAt  *time.Time

    DueAt        *time.Time
    SnoozedUntil *time.Time

    Priority     int
    Temporary    bool

    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type DesktopPresentation struct {
    TodoID       int64
    Mode         DesktopDisplayMode
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

关系：

```text
Todo
 └── DesktopPresentation
      ├── EDGE  → EdgeStack
      ├── CARD  → CardPlacement
      ├── FOCUS
      └── NONE
```

这样可以避免 `Pinned` 与 `DisplayMode` 同时存在造成冲突状态。

---

## 6. Edge Todo 交互规范

### 6.1 默认位置

默认：

```text
屏幕右侧
```

用户可以切换：

```text
左侧
右侧
```

不建议 V1 支持上下边缘。

### 6.2 默认尺寸

推荐：

```text
收起露出宽度：10 ~ 14 px
标签高度：40 ~ 44 px
标签间距：6 px
```

支持密度：

```text
紧凑
标准
宽松
```

默认：

```text
标准
```

### 6.3 Hover

Hover 延迟：

```text
150 ms
```

展开动画：

```text
180 ~ 220 ms
```

展开宽度：

```text
220 ~ 300 px
```

根据标题长度适当变化。

鼠标离开后：

```text
400 ms
```

再开始收起。

避免用户移动鼠标时产生闪烁。

---

## 7. Hover Safe Corridor

必须设计 Hover 安全区域。

例如：

```text
Edge Tab
   ↓
────────────
        \
         \
          Quick Card
```

即使鼠标暂时离开标签本身，只要正在向展开内容移动，就不能立即收起。

建议：

```text
安全时间：400 ~ 600 ms
```

这是体验关键点。

---

## 8. Edge Todo 状态机

状态控制尽量简单。

```text
Hidden
   ↓
Collapsed
   ↓ hover
Expanded
   ↓ click
QuickCard
   ↓ edit
Editing
```

避免增加大量中间状态。

---

## 9. Collapsed 模式

默认只表达：

> 这里存在 Todo。

例如：

```text
┃
┣━━
┣━━
┣━━
┗━━
```

默认不显示完整标题。

提供 Todo 独立选项：

```text
显示标题
```

开启以后可以：

```text
         发布测试环境 ━━━┫
```

默认关闭。

---

## 10. Hover 模式

Hover 只展示必要信息。

例如：

```text
□ 准备周会演示材料      16:00
```

允许显示：

- checkbox
- Title
- Due time
- 简单优先级提示

不显示：

- 完整描述
- 大量按钮
- 项目信息
- 复杂菜单

原则：

> Hover = 看。

---

## 11. Click 模式

点击 Edge Todo 打开 Quick Card。

例如：

```text
┌─────────────────────────┐
│ 准备周会演示材料         │
│                         │
│ 今天 16:00              │
│                         │
│ 整理最新数据             │
│ 优化 PPT                 │
│                         │
│ ✓ 完成   ⏰ 稍后   ✎ 编辑│
└─────────────────────────┘
```

核心动作只有：

```text
完成
稍后
编辑
```

更多操作进入：

```text
···
```

---

## 12. 双击

双击 Todo：

```text
进入快速编辑状态
```

无需打开 Main Window。

---

## 13. Snooze / 稍后

“稍后”属于 V1 核心能力，因为 Quick Card 的核心动作固定为：

```text
完成 / 稍后 / 编辑
```

V1 只实现：

```text
30 分钟
1 小时
明天
```

数据：

```go
SnoozedUntil *time.Time
```

### 13.1 时间规则

```text
30 分钟
→ now + 30min

1 小时
→ now + 1h

明天
→ 系统本地时间下一自然日 09:00
```

例如：

```text
当前：2026-10-04 15:00
选择：明天

SnoozedUntil = 2026-10-05 09:00
```

“明天”不是简单的 `now + 24h`。

V1 固定使用：

```text
09:00
```

以后版本可以增加用户自定义“明天默认恢复时间”。

### 13.2 Snooze 不改变 DueAt

Snooze 只表示：

> 现在先不要在 Edge 中显示。

它不表示：

> 修改截止时间。

例如：

```text
DueAt = 今天 16:00
Snooze = 明天
```

第二天 09:00 恢复后：

```text
DueAt 仍然是昨天 16:00
```

因此该 Todo 应正常显示为：

```text
逾期
```

### 13.3 布局规则

```text
当前时间 < SnoozedUntil
→ Todo 暂时不在 Edge 中显示

当前时间 >= SnoozedUntil
→ Todo 自动重新出现
```

Snooze 不改变：

- `DesktopPresentation.Mode`
- Edge Stack 归属
- `sort_order`
- `DueAt`

Todo 恢复后必须回到原来的 Stack 与原来的顺序。

V1.1 再增加：

```text
今天下午
今晚
下周
自定义日期时间
重复 Snooze
```

---

## 14. Todo 完成、撤销与临时任务

### 14.1 完成状态

点击完成后，数据库立即写入：

```text
completed = true
completed_at = now
```

UI 流程：

```text
点击 checkbox
↓
显示 ✓
↓
显示「已完成」
↓
约 800ms
↓
Todo 从 Edge 淡出
↓
其余 Todo 平滑补位
```

### 14.2 Undo

`800ms` 只是视觉反馈时间，不是撤销有效时间。

建议：

```text
完成动画：约 800ms
Undo 有效期：5 秒
```

完成后短暂显示：

```text
已完成「准备周会材料」    撤销
```

撤销：

```text
completed = false
completed_at = null
```

### 14.3 完成时不删除桌面布局关系

普通 Todo 完成后：

- 不删除 `DesktopPresentation`
- 不删除 `EdgeStackItem`
- 只是不参与 Edge 渲染

这样撤销以后可以自动回到：

- 原来的 Edge Stack
- 原来的显示器
- 原来的左右边缘
- 原来的任务顺序

### 14.4 Temporary Todo

`Temporary = true` 的任务完成时，也不能立即物理删除。

流程：

```text
完成
↓
completed = true
↓
保留 5 秒 Undo 窗口
↓
无人撤销
↓
删除 Temporary Todo
```

为处理程序在 Undo 窗口内退出的情况：

```text
程序启动时
清理 temporary = true AND completed = true 的任务
```

---

## 15. Edge Todo 最大数量

单个 Edge Stack：

```text
最多直接显示 8 条
```

超过以后：

```text
┣━━
┣━━
┣━━
┣━━
┣━━
┣━━
┣━━
┣━━
┣ +5
```

Hover：

```text
+5
```

显示剩余列表。

避免整个屏幕边缘变成标签墙。

---

## 16. 哪些 Todo 进入 Edge

默认：

```text
只有用户主动固定的 Todo。
```

Todo 提供：

```text
固定到桌面
```

另外允许配置自动规则：

```text
○ 仅手动固定
○ 今日任务
○ 今日 + 逾期
○ 高优先级
○ 自定义
```

V1 默认：

```text
仅手动固定
```

---

## 17. Todo 排序

默认排序：

```text
手动排序
↓
截止时间
↓
创建时间
```

规则：

> 用户手动排序后，永远优先尊重用户排序。

支持拖动调整：

```text
任务 A
任务 B
任务 C
```

↓

```text
任务 C
任务 A
任务 B
```

---

## 18. Edge Stack 布局模型

用户拖动的是整个 Edge Stack，而不是每一条 Todo 的绝对屏幕位置。

关系：

```text
Display
   │
   └── EdgeStack
          ├── Todo A / order 10
          ├── Todo B / order 20
          ├── Todo C / order 30
          └── Todo D / order 40
```

### 18.1 EdgeStack

```go
type EdgeStack struct {
    ID        int64

    DisplayID string
    Side      string  // left / right
    Offset    float64 // 0.0 ~ 1.0

    Density   string  // compact / normal / relaxed

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

示例：

```text
display_id = DISPLAY_2
side       = right
offset     = 0.36
```

表示：

> 第二块显示器右侧，高度约 36% 的位置。

不要使用绝对像素保存 Stack 主位置。

### 18.2 EdgeStackItem

```go
type EdgeStackItem struct {
    StackID   int64
    TodoID    int64
    SortOrder int
}
```

拖整个 Stack：

```text
只修改 EdgeStack.display_id / side / offset
```

Todo 拖动排序：

```text
只修改 EdgeStackItem.sort_order
```

Todo Snooze、完成或临时隐藏：

```text
不修改 EdgeStackItem.sort_order
```

因此恢复后必须回到原位置。

---

## 19. 桌面编辑模式

正常情况下：

```text
锁定布局
```

防止误拖。

进入：

```text
整理桌面
```

之后才允许：

- 拖动 Edge Stack。
- 拖动卡片。
- 调整顺序。
- 切换左右屏。
- 调整卡片尺寸。
- 调整边缘位置。

退出编辑模式：

```text
全部锁定。
```

---

## 20. Focus Mode

用户可以进入专注模式。

正常：

```text
┣ Task A
┣ Task B
┣ Task C
```

Focus：

```text
┣ 当前 Task
```

完成后：

```text
自动切换下一条
```

适合持续工作。

---

## 21. 临时 Todo

支持：

```text
完成即删除
```

适合：

```text
等构建结束
等文件下载
下午联系客户
5 点重启服务器
```

避免这类一次性任务永久进入历史。

---

## 22. Display Until

Todo 可设置：

```text
显示到今晚
显示到明天
显示到指定时间
```

时间结束后：

```text
取消桌面展示
```

Todo 本身仍然存在。

---

## 23. Todo Aging

避免长期固定后产生视觉免疫。

可采用极其克制的变化：

```text
Day 1
正常

Day 3
轻微增强边缘线

Overdue
出现小状态点
```

禁止：

- 闪烁。
- 呼吸灯。
- 强烈红色。
- 抖动。
- 频繁弹窗。

---

## 24. Priority 视觉

优先级只使用非常轻微的视觉表达。

例如：

```text
紧急      红色细线
高        橙色细线
普通      蓝色细线
无        灰色
```

仅 2~3 px。

不要整块染色。

---

## 25. 鼠标穿透

普通用户不需要管理这个技术概念。

系统自动处理：

```text
Collapsed
→ 尽可能不影响下面窗口

Hover / Expanded
→ 可交互

Quick Card
→ 正常交互
```

Native 层根据状态切换：

```text
Click Through
NoActivate
TopMost
```

---

## 26. 焦点原则

Sidelet 默认不应抢占用户当前工作的输入焦点。

例如：

```text
用户只是 Hover Edge Todo
↓
IDEA / Chrome 继续保持键盘输入焦点
```

以下状态默认不获取键盘焦点：

```text
Collapsed
Expanded
Quick Card View
```

只有两类明确情况允许 Sidelet 获取键盘焦点：

### 26.1 用户主动进入 Keyboard Interaction Mode

例如：

```text
Ctrl + Alt + T
```

此时允许：

```text
↑ / ↓
Enter
Space
S
E
Esc
```

等 Todo 操作按键生效。

这是：

> 用户明确授权 Sidelet 临时接管键盘操作。

### 26.2 用户进入输入 / 编辑状态

例如：

```text
编辑 Todo 标题
编辑备注
Quick Add 输入
```

此时允许：

```text
Sidelet 获取文本输入焦点
```

退出：

```text
Keyboard Interaction Mode
```

或：

```text
Editing Mode
```

以后，应按第 34 节规则恢复：

```text
Passive Mode
```

并尽可能恢复之前的前台窗口。

因此核心原则修正为：

> **Hover / 查看不抢焦点；显式 Keyboard Interaction Mode 可以临时接管操作键；真正输入 / 编辑时才获取文本输入焦点。**

---


---

## 27. 全屏规则

检测：

```text
视频全屏
游戏
PPT
远程桌面全屏
```

默认：

```text
自动隐藏所有 Overlay
```

设置：

```text
全屏程序时

● 自动隐藏
○ 始终显示
```

---

## 28. 安静模式

提供全局：

```text
安静模式
```

开启：

```text
隐藏全部 Overlay
```

但：

```text
Todo 数据
提醒
固定状态
```

全部保持。

再次关闭：

```text
恢复原布局。
```

适用于：

- 开会。
- 投屏。
- 录屏。
- 演示。

---

## 29. 多显示器与 Edge Stack 迁移规则

### 29.1 Stack 数量限制

V1 全局最多允许：

```text
2 个 EdgeStack Window
```

单显示器规则：

```text
左侧最多 1 个 Stack
右侧最多 1 个 Stack
```

即：

```text
Screen
├── Left Stack
└── Right Stack
```

V1 不允许同一显示器同一侧存在多个 Stack。

多显示器场景也仍然遵循：

```text
整个 App 最多 2 个 Stack
```

例如：

```text
Monitor 1 / Right Stack
Monitor 2 / Right Stack
```

是允许的。

### 29.2 显示器断开

Todo 固定时记录：

```text
display_id
side
offset
```

假设：

```text
Monitor 1
└── Right Stack A

Monitor 2
└── Right Stack B
```

当 Monitor 2 被断开时：

```text
Stack B
→ 自动迁移到主显示器
```

迁移策略：

```text
1. 原 Side 空闲
   → 保持原 Side

2. 原 Side 已被占用
   → 尝试 Opposite Side

3. Opposite Side 也被占用
   → 属于理论异常，需要记录日志并使用安全降级策略
```

由于 V1 全局最多 2 个 Stack，正常情况下第 3 种不会发生。

### 29.3 Offset 定义与最终位置计算

`offset` 不表示 Stack 顶部位置。

V1 明确定义：

> `offset` 表示 EdgeStack **垂直中心点**相对于当前显示器可用工作区（WorkArea）高度的比例位置。

例如：

```text
offset = 0.35
```

表示：

> EdgeStack 的中心点希望位于当前显示器可用工作区高度约 35% 的位置。

计算：

```text
desiredCenterY = workAreaTop + workAreaHeight * offset

stackTop = desiredCenterY - stackHeight / 2
```

但 `offset` 只代表：

> 用户希望 Stack 大致处于什么位置。

它不能作为最终可见性保证。

最终位置必须根据：

```text
显示器 WorkArea
Stack 实际高度
当前 DPI
任务栏占用区域
安全边距
```

重新计算。

建议：

```text
safeMargin = 8 ~ 12 logical px
```

最终：

```text
minTop = workAreaTop + safeMargin

maxTop =
    workAreaBottom
    - stackHeight
    - safeMargin

stackTop =
    clamp(
        stackTop,
        minTop,
        maxTop
    )
```

因此不再使用简单的：

```text
0.10 <= offset <= 0.90
```

作为 Stack 可见性的判断条件。

### 29.4 Overlay 边界校验

不仅 EdgeStack 本体需要边界检查。

以下 Overlay 都必须进行 WorkArea 边界校验：

```text
Collapsed EdgeStack
Expanded Edge Todo
Quick Card
Floating Card（V1.1）
```

建议抽象统一几何方法：

```go
type Rect struct {
    X      float64
    Y      float64
    Width  float64
    Height float64
}

func EnsureVisible(rect Rect, workArea Rect, safeMargin float64) Rect
```

平台层负责提供：

```text
WorkArea
DPI
Display bounds
Taskbar / Dock 后的可用区域
```

布局层负责计算最终 Rect。

#### Expanded Edge Todo

右侧 Edge Todo：

```text
默认向左展开
```

左侧 Edge Todo：

```text
默认向右展开
```

如果预计展开宽度超出 WorkArea：

```text
优先缩小到允许的最大宽度
```

不能让展开内容跑出当前显示器。

#### Quick Card

Quick Card 默认：

```text
优先贴着当前 Todo 的屏幕内侧出现
```

然后按以下顺序修正：

```text
1. 计算默认位置

2. 如果底部越界
   → 向上偏移

3. 如果顶部越界
   → 向下偏移

4. 如果水平越界
   → 向屏幕内部调整

5. 最终调用 EnsureVisible
```

最终要求：

> Quick Card 必须完整落在当前显示器 WorkArea 内。

### 29.4.1 内容超过 WorkArea 时的降级策略

边界校验不能假设 Overlay 一定小于 WorkArea。

如果出现：

```text
stackHeight + 2 × safeMargin > workAreaHeight
```

则：

```text
maxTop < minTop
```

此时单纯 Clamp 无法保证整个 Stack 可见。

V1 必须采用降级策略。

#### EdgeStack

优先减少直接显示的 Todo 数量。

例如原本：

```text
Todo A
Todo B
Todo C
Todo D
Todo E
Todo F
Todo G
Todo H
```

如果当前 WorkArea 无法完整容纳：

```text
Todo A
Todo B
Todo C
Todo D
Todo E
+3
```

其余任务进入：

```text
+N
```

聚合入口。

规则：

```text
根据：
WorkAreaHeight
safeMargin
当前 Density
TodoItemHeight
TodoGap

动态计算 maxVisibleItems
```

其中：

```text
maxVisibleItems >= 1
```

并且：

> `+N` 本身必须占用一个可见 Item 的高度预算。

因此实际计算应先为：

```text
+N
```

预留空间，再决定直接显示多少 Todo。

不能通过：

```text
压缩 Todo 到不可读高度
```

来强行塞入屏幕。

#### Expanded Edge Todo

如果单条 Todo 展开后的内容尺寸超过 WorkArea：

```text
优先限制最大宽度 / 最大高度
```

并保留核心信息：

```text
Title
DueAt
核心操作
```

较长描述可以截断或进入 Quick Card。

#### Quick Card

如果 Quick Card 自然高度超过 WorkArea：

```text
限制 maxHeight
```

建议：

```text
maxHeight =
workAreaHeight
- 2 × safeMargin
```

超出的内容：

```text
在 Quick Card 内部滚动
```

不能：

```text
让整个 Quick Card 越出屏幕
```

并且：

```text
标题区
核心操作区
```

应尽量保持可见，优先让中间内容区域滚动。

#### Spike 必测场景

Phase 0 Spike 必须增加：

```text
小分辨率显示器
高 DPI
8 条 Edge Todo
大字号 / 宽松 Density
超长 Todo 标题
超高 Quick Card 内容
Taskbar 占用较大 WorkArea
```

验收要求：

```text
Stack 不越界
至少保留 1 条 Todo + 必要的 +N 聚合入口
Quick Card 不越界
Quick Card 超高时内部滚动
不存在 maxTop < minTop 导致的非法位置
```

---

### 29.5 显示器重新连接

V1 明确规定：

> 显示器重新连接后，不自动把 Stack 移回原显示器。

原因：

- 用户可能已经在新位置工作一段时间。
- 自动跳回会造成不可预测的界面变化。
- V1 优先保证稳定而不是自动恢复复杂布局。

用户可以进入：

```text
整理桌面
```

手动重新安排。

未来可在 V1.1+ 增加：

```text
记住显示器布局
布局快照
```

---

## 30. DPI 与分辨率变化

必须正确处理：

```text
100%
125%
150%
175%
200%
```

标签尺寸使用逻辑单位。

程序启动 / 显示器变化 / DPI 变化时：

```text
重新校验窗口位置
```

确保窗口始终处于屏幕可见范围。

---

## 31. Floating Card

Todo 可独立转换为半透明桌面卡片。

例如：

```text
┌────────────────────┐
│ 发布测试版本        │
│                    │
│ 今天 17:00         │
│                    │
│ □ 后端             │
│ □ H5              │
│ □ 小程序           │
└────────────────────┘
```

默认：

```text
TopMost
```

支持：

```text
透明度
位置
尺寸
锁定
```

---

## 32. Floating Ball

用户可以隐藏 Edge Todo，仅保留：

```text
✓ 5
```

Hover：

```text
显示前几个 Todo。
```

Click：

```text
打开快速 Todo Panel。
```

右键：

```text
快速添加
显示主窗口
安静模式
设置
退出
```

---

## 33. 全局 Quick Add

建议默认快捷键：

```text
Ctrl + Shift + Space
```

弹出：

```text
┌──────────────────────────────┐
│ + 输入待办...                │
└──────────────────────────────┘
```

例如输入：

```text
明天下午3点 联系客户
```

简单规则解析：

```text
Title = 联系客户
DueAt = 明天 15:00
```

V1：

> 使用规则解析即可，不需要 AI。

---

## 34. 键盘操作与输入模式

Sidelet 默认不得消费用户正在其他应用中输入的普通键盘事件。

核心原则：

> **只有用户明确进入 Keyboard Interaction Mode 后，Sidelet 才允许接管普通按键。**

### 34.1 Passive Mode

以下视觉状态默认都属于 Passive Mode：

```text
Collapsed
Expanded
Quick Card View
```

此时：

```text
不监听普通字母键
不监听 Space
不抢 keyboard focus
```

例如：

```text
用户正在 IDEA 输入：

service
```

即使鼠标 Hover 到 Edge Todo：

```text
S
E
Space
↑
↓
```

也不能被 Sidelet 截获。

### 34.2 Keyboard Interaction Mode

Keyboard Interaction Mode 必须由明确用户动作触发。

V1 可以使用独立全局快捷键，例如：

```text
Ctrl + Alt + T
```

进入 Keyboard Interaction Mode 后，才启用：

```text
↑ / ↓      切换 Todo

Enter      打开 / 确认

Space      完成

S          稍后

E          编辑

Esc        退出 Keyboard Interaction Mode
```

快捷键后续允许用户修改。

### 34.3 焦点获取

进入 Keyboard Interaction Mode 后：

```text
Sidelet 获取键盘输入焦点
```

同时应记录进入前的前台窗口：

```text
previousForegroundWindow
```

例如：

```text
IDEA
↓
Ctrl + Alt + T
↓
Sidelet Keyboard Interaction Mode
```

只有在这个状态下：

```text
Space / S / E / ↑ / ↓
```

才具有 Sidelet 内部含义。

### 34.4 Esc 退出规则

用户按下：

```text
Esc
```

必须执行：

```text
退出 Keyboard Interaction Mode
↓
关闭仅为键盘模式临时展开的 UI
↓
释放 Sidelet 键盘输入焦点
↓
恢复 Passive Mode
↓
尽可能恢复 previousForegroundWindow
```

例如：

```text
IDEA
↓
进入 Sidelet Keyboard Mode
↓
操作 Todo
↓
Esc
↓
继续回到 IDEA 输入
```

### 34.5 点击外部区域

如果 Sidelet 当前处于 Keyboard Interaction Mode，而用户点击 Sidelet 外部：

```text
立即退出 Keyboard Interaction Mode
```

然后恢复：

```text
Passive
NoActivate
非抢焦点
```

状态。

### 34.6 Editing Mode

真正进入 Todo 编辑输入框时：

```text
允许正常获取键盘焦点
允许消费文本输入
```

结束编辑：

```text
Enter
Esc
点击外部区域
```

应根据当前交互上下文返回：

```text
KeyboardActive
```

或者：

```text
Passive
```

### 34.7 状态模型

Edge Todo 的视觉状态与输入状态必须解耦。

视觉状态：

```text
VisualState

Hidden
Collapsed
Expanded
QuickCard
Editing
```

输入状态：

```text
InputMode

Passive
KeyboardActive
Editing
```

例如：

```text
Expanded + Passive
```

表示：

> Todo 已经展开，但仍然不消费用户键盘输入。

而：

```text
Expanded + KeyboardActive
```

表示：

> 用户已经主动进入键盘操作状态，可以使用快捷操作键。

这样避免将：

```text
Hover 展开
```

错误等同于：

```text
获得键盘焦点
```


---

## 35. 托盘

主窗口关闭：

```text
程序继续运行。
```

Tray：

```text
快速添加
显示主界面
安静模式
隐藏全部
设置
退出
```

只有：

```text
退出
```

才真正结束程序。

---

## 36. 通知与 Edge Todo 的关系

Edge Todo：

```text
视觉提醒
```

Windows Notification：

```text
主动打断
```

二者不应该默认重复出现。

如果 Todo 已经固定：

```text
默认只进行视觉提醒。
```

只有明确设置：

```text
提醒我
```

才发送系统通知。

---

## 37. 数据模型

### 37.1 Todo

```go
type Todo struct {
    ID           int64
    Title        string
    Description  string

    Completed    bool
    CompletedAt  *time.Time

    DueAt        *time.Time
    SnoozedUntil *time.Time

    Priority     int
    Temporary    bool

    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### 37.2 DesktopPresentation

```go
type DesktopPresentation struct {
    TodoID       int64
    Mode         DesktopDisplayMode
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### 37.3 EdgeStack

```go
type EdgeStack struct {
    ID        int64
    DisplayID string
    Side      string
    Offset    float64
    Density   string

    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 37.4 EdgeStackItem

```go
type EdgeStackItem struct {
    StackID   int64
    TodoID    int64
    SortOrder int
}
```

### 37.5 核心状态规则

```text
DisplayMode = NONE
→ 不显示桌面 Overlay

DisplayMode = EDGE
→ 必须存在有效 EdgeStackItem

Completed = true
→ 暂不参与桌面渲染，但保留 Presentation / StackItem

SnoozedUntil > now
→ 暂不参与 Edge 渲染，但保留 Presentation / StackItem

Temporary = true AND Completed = true
→ Undo 窗口结束后物理删除
```

---

## 38. SQLite

主要业务数据：

```text
todos
desktop_presentations
edge_stacks
edge_stack_items
reminders
schema_migrations
```

未来启用 Floating Card 时增加：

```text
desktop_card_placements
```

V1 必须从：

```text
schema version 1
```

开始维护 migration。

关键约束：

```text
edge_stack_items(todo_id)
→ 同一 Todo 在 EDGE 模式下只归属一个 Stack

desktop_presentations(todo_id)
→ 每条 Todo 最多一个桌面呈现记录
```

避免未来升级数据库结构困难。

---

## 39. SQLite 与 settings.json 职责边界

必须避免同一个状态在 SQLite 与 `settings.json` 中重复保存。

统一原则：

> **跟 Todo 和桌面内容布局有关 → SQLite。**

> **跟整个应用的全局偏好和系统行为有关 → settings.json。**

### 39.1 SQLite

SQLite 负责：

```text
Todo 内容
Todo 状态
Todo 排序
Todo DisplayMode
Todo 属于哪个 EdgeStack
EdgeStack 所在显示器
EdgeStack 左 / 右
EdgeStack offset
EdgeStack density
Snooze
Reminder
```

对应主要表：

```text
todos
desktop_presentations
edge_stacks
edge_stack_items
reminders
schema_migrations
```

例如：

```text
EdgeStack.offset = 0.36
```

只能存在 SQLite。

不能同时写入 `settings.json`。

### 39.2 settings.json

只负责应用级偏好：

```text
主题
主窗口位置与尺寸
新建 EdgeStack 的默认 Side
新建 EdgeStack 的默认 Density
Hover Delay
Collapse Delay
全局快捷键
全屏自动隐藏
开机启动
全局外观偏好
```

示例：

```json
{
  "theme": "system",
  "mainWindow": {
    "width": 960,
    "height": 680,
    "x": 120,
    "y": 80
  },
  "edge": {
    "defaultSide": "right",
    "defaultDensity": "normal"
  },
  "interaction": {
    "hoverDelay": 150,
    "collapseDelay": 400
  },
  "hotkeys": {
    "quickAdd": "Ctrl+Shift+Space"
  },
  "fullscreen": {
    "autoHide": true
  },
  "startup": {
    "enabled": true
  }
}
```

这里的：

```text
defaultSide
defaultDensity
```

只用于：

> 创建新的 EdgeStack 时的默认值。

已经存在的 Stack：

```text
display_id
side
offset
density
```

全部以 SQLite 为准。

### 39.3 最终边界

```text
SQLite
= 用户内容 + 内容呈现 + 内容布局

settings.json
= App 全局偏好 + 系统行为 + Main Window 状态
```

---

## 40. 平台抽象

建议：

```text
internal/platform/
├── platform.go
│
├── windows/
│   ├── window.go
│   ├── hotkey.go
│   ├── tray.go
│   ├── autostart.go
│   ├── fullscreen.go
│   └── display.go
│
└── darwin/
    ├── window.go
    ├── hotkey.go
    ├── tray.go
    ├── autostart.go
    └── display.go
```

上层：

```go
platform.SetAlwaysOnTop()
platform.SetClickThrough()
platform.RegisterHotkey()
platform.GetDisplays()
platform.IsFullscreenApp()
```

禁止业务层直接操作：

```text
HWND
user32.dll
WS_EX_xxx
```

---

## 41. 推荐目录结构

```text
sidelet/
├── main.go
├── app.go
├── wails.json
│
├── internal/
│   ├── todo/
│   │   ├── model.go
│   │   ├── repository.go
│   │   └── service.go
│   │
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── migrations/
│   │
│   ├── platform/
│   │   ├── platform.go
│   │   ├── windows/
│   │   └── darwin/
│   │
│   ├── config/
│   └── reminder/
│
├── frontend/
│   └── src/
│       ├── components/
│       │   ├── EdgeTodo.svelte
│       │   ├── EdgeTodoStack.svelte
│       │   ├── QuickCard.svelte
│       │   ├── FloatingBall.svelte
│       │   ├── FloatingCard.svelte
│       │   └── TodoItem.svelte
│       │
│       ├── stores/
│       ├── routes/
│       └── App.svelte
│
└── build/
```

---

## 42. 窗口架构

不要把所有 UI 塞进同一个窗口，也不要为每条 Todo 创建独立 WebView。

推荐：

```text
Wails App
│
├── Main Window
│
├── EdgeStack Window × 1~2
│   ├── EdgeTodo A
│   ├── EdgeTodo B
│   ├── EdgeTodo C
│   └── ...
│
├── Quick Card Window × 0~1
│
└── Native Platform Layer
    ├── TopMost
    ├── NoActivate
    ├── ClickThrough
    ├── Hotkey
    ├── Tray
    ├── Displays
    └── Fullscreen Detection
```

核心规则：

> 一个 Edge Stack 对应一个 Overlay Window，而不是一个 Todo 对应一个 Window。

例如 8 条 Edge Todo：

```text
仍然只使用一个 EdgeStack Window
```

Hover 展开某条 Todo 时：

```text
由 Svelte 在同一 EdgeStack Window 内更新对应 Item
```

不新建窗口。

正常运行状态建议控制为：

```text
Main Window
→ 默认隐藏

EdgeStack Window
→ 1 个，最多 2 个

Quick Card
→ 按需出现

Tray
→ Native
```

这样可以明显降低 WebView2 的常驻资源开销。

---

## 43. 动画职责

Svelte：

```text
checkbox
fade
card 内容
hover 内容动画
```

Native：

```text
窗口位置
屏幕边缘吸附
窗口层级
窗口移动
```

不要通过 WebView 做所有窗口级动画。

---

## 44. 性能目标与验证

常驻软件必须把性能当作产品能力，但 V1 不在实测前承诺固定内存上限。

原来的：

```text
内存尽量 < 100 MB
```

调整为：

> `<100 MB` 作为优化方向，而不是未经验证的 V1 硬门槛。

### 44.1 第一轮性能预算

```text
Idle CPU
目标 < 0.5%

无动画时
禁止高频轮询与持续重绘

Hover / 展开动画
目标 60fps，肉眼无明显掉帧

数据库
事件驱动写入，禁止高频轮询写数据库

内存
先建立真实性能 baseline，再冻结验收指标
```

### 44.2 Edge Window Spike 实测场景

正式开发前至少测试：

```text
场景 A
程序启动，Main Window 隐藏

场景 B
1 个 EdgeStack
8 条 Todo

场景 C
Collapsed

场景 D
Hover / Expanded

场景 E
Quick Card 打开

场景 F
空闲运行 10 分钟

场景 G
双显示器 + 2 个 EdgeStack
```

记录：

```text
主进程 Working Set
WebView2 相关进程 Working Set
总 Private Bytes
Idle CPU
Hover CPU 峰值
动画 FPS
首次展开延迟
```

如果实测 Wails + WebView2 稳定占用高于 100 MB，但：

```text
CPU 接近 0
响应速度良好
动画顺滑
长期运行稳定
```

则不应单纯为了低于 100 MB 强行更换技术栈。

最终性能验收指标由 Spike 数据决定。

---

## 45. 数据安全

本地优先。

V1：

```text
不要求登录
不要求账号
不要求云同步
```

用户数据：

```text
SQLite 本地保存
```

支持：

```text
JSON 导出
CSV 导出
```

用户可以随时带走自己的数据。

---

## 46. 隐私

Todo 可能包含：

```text
客户名称
项目名称
工作内容
内部事项
```

需要预留：

```text
Privacy Mode
```

未来可以实现：

```text
屏幕共享时隐藏内容
投屏时隐藏
只显示 Edge 标记、不显示文字
```

V1 可以先提供：

```text
安静模式
```

---

## 46.1 版本交付清单

下表是版本范围的唯一基准。

| 功能 | V1 | V1.1 | V1.2+ |
|---|---:|---:|---:|
| Main Window | ✅ |  |  |
| Todo CRUD | ✅ |  |  |
| SQLite + migration | ✅ |  |  |
| Edge Todo | ✅ |  |  |
| EdgeStack | ✅ |  |  |
| 局部 Mouse Hit Region | ✅ |  |  |
| Hover 展开 | ✅ |  |  |
| Quick Card | ✅ |  |  |
| Keyboard Interaction Mode | ✅ |  |  |
| 完成 / Undo | ✅ |  |  |
| 基础 Snooze：30m / 1h / 明天 | ✅ |  |  |
| Temporary Todo | ✅ |  |  |
| 手动固定到 Edge | ✅ |  |  |
| Todo 手动排序 | ✅ |  |  |
| 左 / 右吸附 | ✅ |  |  |
| Tray | ✅ |  |  |
| Quick Add | ✅ |  |  |
| 开机启动 | ✅ |  |  |
| 全屏自动隐藏 | ✅ |  |  |
| JSON / CSV 导出 | ✅ |  |  |
| Floating Ball |  | ✅ |  |
| Floating Card |  | ✅ |  |
| Focus Mode |  | ✅ |  |
| 高级 Snooze |  | ✅ |  |
| Display Until |  | ✅ |  |
| 自动固定：今日任务 |  | ✅ |  |
| 自动固定：今日 + 逾期 |  | ✅ |  |
| 自动固定：高优先级 |  | ✅ |  |
| Priority 视觉增强 |  | ✅ |  |
| Windows Notification |  | ✅ |  |
| 布局快照 |  | ✅ |  |
| Privacy Mode 增强 |  | ✅ |  |
| macOS |  |  | ✅ |
| Project / Tags |  |  | ✅ |
| Repeat Todo |  |  | ✅ |
| 复杂提醒 |  |  | ✅ |
| AI / 自然语言增强 |  |  | ✅ |

说明：

- 产品总体模型可以讨论后续模式，但 V1 代码只实现 `NONE / EDGE`。
- `Display Until` 整体属于 V1.1，不进入 V1 数据模型。
- 自动固定规则属于 V1.1，V1 只支持手动固定。
- Windows Notification 属于 V1.1。
- JSON / CSV 导出属于 V1。

---

## 47. V1 MVP

第一阶段只验证核心价值。

开发顺序：

```text
Phase 0
Edge Window Spike / 架构可行性验证

Phase 1
Todo / SQLite / Main Window

Phase 2
Edge Todo 核心交互

Phase 3
Quick Card / Keyboard Interaction / Snooze / Undo

Phase 4
系统集成能力

Phase 5
导出与稳定性收尾
```

V1 必须实现：

```text
1. Edge Window Spike
2. 局部 Mouse Hit Region
3. Hover 不抢焦点
4. 透明区域点击/滚轮穿透
5. Overlay 超出 WorkArea 时的降级策略
6. Todo CRUD
7. SQLite + migration
8. Main Window
9. DesktopPresentation（仅 NONE / EDGE）
10. EdgeStack / EdgeStackItem
11. 手动固定 Todo 到 Edge
12. Edge Todo
13. Hover 展开
14. Quick Card
15. Keyboard Interaction Mode
16. Passive / KeyboardActive / Editing 输入状态切换
17. Esc 退出键盘模式并恢复前台应用
18. 完成 Todo
19. 基础 Snooze：30 分钟 / 1 小时 / 明天 09:00
20. Undo
21. Temporary Todo 完成后延迟删除
22. Edge Stack 拖动
23. Edge Todo 手动排序
24. Stack 超高时动态减少直接显示数量并使用 +N
25. Quick Card 超高时限制高度并内部滚动
26. 左右边缘切换
27. 托盘
28. 全局 Quick Add
29. 开机启动
30. 全屏自动隐藏
31. 配置自动保存
32. DPI / 显示器变化后的布局校验
33. 显示器断开时 Stack 自动迁移
34. JSON 导出
35. CSV 导出
```

---


---

## 48. V1.1

核心体验稳定以后增加：

```text
Floating Ball
Floating Card
Focus Mode

高级 Snooze
    - 今天下午
    - 今晚
    - 下周
    - 自定义日期时间

Display Until

自动固定规则
    - 今日任务
    - 今日 + 逾期
    - 高优先级

Priority 视觉增强
Windows Notification
密度增强
多显示器增强
布局快照
记住显示器布局
Privacy Mode 增强
```

---

## 49. V1.2+

之后再考虑：

```text
项目
标签
重复任务
复杂提醒
自然语言增强
macOS
统计
历史分析
插件能力
```

---

## 50. 明确暂不开发

V1 不做：

```text
账号体系
云同步
团队协作
多人 Todo
AI Agent
聊天
复杂看板
复杂项目管理
日历系统
文档系统
复杂统计
```

避免产品失焦。

---

## 50.1 V1 开发冻结规则

以下规则在进入正式开发后视为 V1 基线，不应由实现阶段随意改变：

```text
1. V1 只实现 NONE / EDGE。
2. 一个 EdgeStack 对应一个 Overlay Window。
3. 一条 Todo 不对应一个独立 WebView Window。
4. V1 全局最多 2 个 EdgeStack。
5. 单显示器每侧最多 1 个 EdgeStack。
6. Snooze 不改变 DueAt。
7. “明天”固定恢复到下一自然日 09:00。
8. 完成 / Snooze 不删除 EdgeStackItem。
9. Temporary Todo 只有 Undo 窗口结束后才物理删除。
10. SQLite 保存 Todo / Presentation / Stack / Stack 布局。
11. settings.json 只保存 App 级偏好与 Main Window 状态。
12. 显示器断开时 Stack 自动迁移。
13. 显示器恢复时 V1 不自动迁回。
14. Hover 不抢焦点。
15. 透明区域不能挡住下层应用。
16. offset 表示 EdgeStack 垂直中心点的相对位置，不表示顶部坐标。
17. 最终 Stack 位置必须结合 WorkArea、实际高度、DPI 和安全边距重新 Clamp。
18. Expanded Todo 与 Quick Card 必须经过 WorkArea 边界校验。
19. Passive Mode 下 Sidelet 不消费普通键盘输入。
20. 只有用户主动进入 Keyboard Interaction Mode 后才启用 Space / S / E / ↑ / ↓ 等快捷操作。
21. Esc 必须退出 Keyboard Interaction Mode，并尽可能恢复之前的前台应用。
22. 当 EdgeStack 高度超过 WorkArea 时，必须减少直接显示数量，其余进入 +N，不能仅依赖 Clamp。
23. 当 Quick Card 高度超过 WorkArea 时，必须限制高度并在内容区域内部滚动。
24. Expanded Todo / Quick Card / EdgeStack 都必须通过统一 WorkArea 边界校验。
25. Spike 必须覆盖内容尺寸大于 WorkArea 的极端场景。
26. Spike 未通过前不进入正式业务开发。
```

---

## 51. 最关键的核心流程

```text
创建 Todo
    ↓
勾选「固定到桌面」
    ↓
Edge Todo 出现在屏幕边缘
    ↓
用户继续正常工作
    ↓
鼠标划过 Edge Todo
    ↓
Todo 展开
    ↓
点击
    ↓
Quick Card
    ↓
完成 / 稍后 / 编辑
    ↓
Todo 状态更新
```

如果这一条体验足够顺滑：

> 产品核心就成立。

---

## 52. 产品最终判断标准

这个产品不应该被评价为：

> 一个漂亮的 Todo 软件。

真正目标应该是：

> 我几乎感觉不到它在运行，但重要事情一直没有从我的视野中消失。

设计任何功能时，都用这三个问题判断：

1. 它是否让 Todo 更容易被感知？
2. 它是否减少用户进入主程序的次数？
3. 它是否增加了桌面干扰？

如果第三项明显增加，而前两项没有明显收益：

> 不做。

---

## 53. 产品核心关键词

最终建议固定为：

**Glance**

一眼感知。

**Hover**

需要时展开。

**Act**

就地完成。

以及：

> Calm by default. Useful on demand.

默认安静，需要时出现。
