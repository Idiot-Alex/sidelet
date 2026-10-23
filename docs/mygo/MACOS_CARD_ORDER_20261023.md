# macOS 卡片输入层防遮挡

2026-10-23，build 42，包含 build 41 的主界面布局优化。

## 问题与修复

用户报告点击浮动卡片的“稍后”或其他组件后整张卡片空白，主窗口正常。当前日常运行路径为 `/Applications/Sidelet.app`，读取包信息确认仍是 build 33；本轮没有替换该安装或访问日常任务资料。

卡片使用 MyGo 窗口绘制，下方另有带不透明底色的 NSPanel 接收鼠标，避免后台悬停 / 点击打断当前应用。禁止面板成为 key window 不等于禁止它被置前；面板一旦位于绘制窗口上方，会盖住全部内容，原适配层只在首次显示时设置上下关系。

- 输入视图实现 `shouldDelayWindowOrderingForEvent:`，鼠标按下时调用 `preventWindowOrdering`，抑制默认点击置前。
- 按下 / 松开后恢复输入层位于绘制窗口下方；显示及尺寸改变后也恢复顺序。普通鼠标移动和拖动不额外重排窗口。
- 不改变完成、稍后时间、草稿或 SQLite；编辑时仍隐藏被动输入层并获取键盘焦点，取消后恢复原窗口焦点。

## 验证与边界

静态三主题稍后界面正常；应用内 AppKit 定向点击也未自然复现层级倒置。新增故障注入明确将自身不透明输入层提到卡片前方：修复前检查失败（绘制窗口顺序 2，输入层顺序 1）；修复后点击能恢复。该结果证明防遮挡与恢复有效，不冒充已复现用户的真实鼠标触发过程。

- macOS 原生诊断共 18 项通过，包括实际 AppKit 事件分发的稍后 / 返回 / 编辑 / 取消、无尺寸变化的正文点击恢复、被动焦点、输入区域、拖动与清理。仅操作自建的合成窗口，无全局输入注入。证据在 `build/results/card-order-fixed-20261023/native-check.json`；修复前失败证据在 `build/results/card-raised-before-20261023/native-check.json`。
- 全量回归、race、vet 通过。卡片渲染预览增加稍后状态，三主题的阅读 / 稍后 / 编辑均可导出。
- 外部定向事件未进入测试进程，系统区域截图不可用；两者未计为通过。原生诊断及窗口内容捕获不代表物理鼠标、屏幕合成截图、多屏或输入法验收。

最终 macOS ARM64 build 42 已从干净源码 `013ba83` 打包并通过 ad-hoc 签名验证。再次使用最终包运行上述 18 项原生检查，全部通过并正常退出；源码信息与构建号已核对，证据在 `build/results/card-order-macos-build42/verified.json`，产物为 `build/bin/Sidelet.app`。

build 42 的 [Windows 补充构建](https://github.com/Idiot-Alex/sidelet/actions/runs/39836025277) 两架构已全部通过，源码 `013ba83`：x64 3 分 9 秒，ARM64 4 分 53 秒，包含原生测试、打包运行和安装器生命周期。这次生产修复仅修改 macOS 输入适配；本轮未下载 build 42 Windows 产物另作本地复验。

执行原生诊断：

```sh
build/bin/Sidelet.app/Contents/MacOS/sidelet \
  -native-main=false -native-check -probe \
  -output build/results/card-order-check -quit-after 15s
```

预览命令：

```sh
SIDELET_CARD_VISUAL_DIR="$PWD/build/results/card-order-previews" \
bash scripts/go.sh test ./cmd/sidelet -run '^TestCardVisualArtifacts$' -count=1
```

Apple API 依据：[延迟窗口置前](https://developer.apple.com/documentation/appkit/nsview/shoulddelaywindowordering%28for%3A%29)、[阻止鼠标按下的默认置前](https://developer.apple.com/documentation/appkit/nsapplication/preventwindowordering%28%29?language=objc)、[可见窗口的前后顺序](https://developer.apple.com/documentation/appkit/nswindow/windownumbers%28options%3A%29?language=objc)。
