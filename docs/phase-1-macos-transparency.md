# macOS 侧栏透明背景修复

2026-10-05，0.1.0 构建 3。用户确认：保留边缘细窄任务标记，悬停时展开；没有任务内容的侧栏区域不绘制背景。

## 原因与修复

此前只将 NSPanel 设为非 opaque、背景透明、无阴影。WKWebView 仍保留默认页面底色，因此鼠标能够穿透的区域在视觉上仍可能是一整块白底。Wails beta.27 默认构建中的 WebView 透明函数为空操作；原生穿透测试使用普通 NSView，不能证明 WebKit 像素透明。

现在在 Sidelet 的 Overlay 绑定处关闭 WKWebView `drawsBackground`，同时清空 `underPageBackgroundColor`。只作用于 Stack 和 Quick Card，主任务 / 设置窗口保持原来的行为；不全局启用 Wails 的其他私有 Mac API。`drawsBackground` 是 WebKit 兼容性键，未来系统更新需要持续回归；设置失败会返回明确错误。前端的 `html`、`body`、`#app` 也显式保持透明。

原生诊断新增窗口 opaque、背景 alpha、WebView 底色开关，能够核对实际安装版。细窄标记、任务内容、Hover 展开逻辑及原有接收面板命中区域保持原行为。

## 验证

新增 `scripts/test-macos-transparency.sh`，使用真实 WKWebView、项目 CSS 和透明页面渲染独立测试图，检查 alpha 通道，不截取其他应用或桌面。已加入 `npm run test:regression`。

- 恢复修复前的绑定实现运行同一测试：空白、内容、空圆角的 alpha 均为 1，准确触发三项失败。
- 修复后：空白 alpha=0，任务内容 alpha=1，空圆角 alpha=0；8 项透明度断言通过。
- 原有 29 项原生路由 / 生命周期断言、19 项前端测试、Go race 与 vet 通过。
- macOS 构建、签名校验与 DMG 校验通过；安装版的 Stack / Quick Card 均报告 `webviewDrawsBackground=false`、`windowOpaque=false`、背景 alpha=0。
- 假任务实例已退出，恢复正常安装版。正常数据库各表、设置前后相同，完整性检查为 `ok`。

证据位于 `build/results/transparency-native-20261005/`，回归与打包日志分别为 `build/results/transparency-regression.log`、`build/results/transparency-package.log`。Sky 提供的窗口 JPEG 不保留 alpha 通道，不用其白色合成底判断透明与否；透明度证据来自原生像素回归和实际安装版配置。该修复不将此前待验收的真实鼠标完整流程、多屏 / Spaces 自动升级为通过。

已更新 `/Applications/Sidelet.app` 和 `build/releases/Sidelet-0.1.0-local-arm64.dmg`，构建号为 3。原构建 2 应用备份保留在证据目录中。
