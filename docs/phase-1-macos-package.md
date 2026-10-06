# macOS 本地安装包 · 0.1.0

当前产物已更新为构建 13，[全窗口验收后撤回主窗口隐藏视口试验](phase-1-full-window-memory.md)，保留 [侧栏空白视口优化](phase-1-stack-viewport-memory.md)，保留 [Quick Card 按需加载](phase-1-quick-card-memory.md)，保留 [主窗口按需加载与内存试验记录](phase-1-memory-lifecycle.md)，包含 [Quick Add](phase-1-quick-add.md)、[JSON / CSV 导出](phase-1-export.md)、[Dock 图标与设置开关](phase-1-macos-dock.md)、[UI 与三套主题](phase-1-ui-themes.md)，并保留 [侧栏透明背景修复](phase-1-macos-transparency.md)。构建 13 已安装到 `/Applications/Sidelet.app`，当前两条任务和设置在升级前后精确相同。下方首次打包 / 升级验证及原稳定性记录对应构建 2，保留为历史证据。

2026-10-06，Apple Silicon / macOS 13+。此次产物是可在本机安装验证的本地预览版；原有 P0 人工验收范围保持不变。

当前构建 13 的验收与回退见 [全窗口记录](phase-1-full-window-memory.md)，已撤回的构建 11 试验见 [主窗口隐藏视口历史记录](phase-1-hidden-main-memory.md)，此前构建 10 的内存对比见 [侧栏视口优化记录](phase-1-stack-viewport-memory.md)，此前 Quick Card 对比见 [构建 9 记录](phase-1-quick-card-memory.md)，构建 8 的结果见 [主窗口按需加载记录](phase-1-memory-lifecycle.md)。构建 7 的空闲 CPU、快速添加 / 设置 / 任务操作、恢复观察与内存原因分析见 [构建 7 稳定性记录](phase-1-build7-stability.md)；同一构建 2 二进制的历史测试保留在 [安装版稳定性记录](phase-1-macos-stability.md)。

## 产物与重建

- `build/releases/Sidelet-0.1.0-local-arm64.dmg`
- `build/releases/Sidelet-0.1.0-local-arm64.dmg.sha256`
- 应用：`build/bin/Sidelet.app`，本次已从 DMG 安装到 `/Applications/Sidelet.app`。
- 镜像中包含 Sidelet、指向 `/Applications` 的安装入口和中文使用说明。

```sh
npm run package:macos
cd build/releases
shasum -a 256 -c Sidelet-0.1.0-local-arm64.dmg.sha256
```

打包命令先完成前端检查、生产构建、任务 / 设置 / 提醒 / 导出 / Quick Add / 串联 / 控制器测试、Go vet；随后在新临时目录构建应用，生成图标、Info.plist、临时签名并验证。最后制作压缩只读 UDZO / HFS+ 镜像，运行 `hdiutil verify`，成功后发布到 `build/releases` 并计算 SHA-256。临时产物退出时清除，镜像不包含任务库、缓存、测试日志或工具链。

## 标识与版本

用户可见名称由 Sidelet Spike 改为 Sidelet，主程序改为 `Contents/MacOS/sidelet`。原 `io.sidelet.spike` bundle ID 保留，以延续系统识别；SQLite 与 `settings.json` 继续使用 `~/Library/Application Support/Sidelet/`，无需导入。保留的旧开发 `.app` 可用作回退，使用时应退出其他实例。

`packaging/macos/app.json` 定义当前版本与 bundle 元数据（现为 0.1.0、构建 13；首次打包为构建 2）。构建时将版本注入可执行程序，设置页和 `sidelet -version` 显示同一版本。SVG 是图标唯一绘图源，Swift / AppKit 离线渲染 16–1024 像素图标并用 `iconutil` 生成 ICNS，不增加运行时依赖。

历史验收文档保留当时应用名称；当前 README、验收启动脚本及进程工具更新到新路径。进程分析仍识别历史 `sidelet-spike` 样本，采样时从真实进程路径计算二进制哈希，避免把安装版误记为构建目录中的版本。

## 安装验收

本轮证据：`build/results/package-native-20261005/`。

- DMG 校验与挂载通过，镜像内应用签名完整；Applications 链接指向正确目录。
- Finder 能显示新名称及图标，截图见 `dmg-finder.jpg`。
- 从镜像复制到原本不存在的 `/Applications/Sidelet.app`，安装版签名校验通过，实际进程路径指向该位置。
- 使用旧版本的两条任务建立隔离升级资料；安装版启动后，任务、展示关系、布局、排序、提醒共五张表及设置逐项相同。UI 保持 B、A 顺序、左侧约 45% 位置与保存的偏好。
- 正常资料目录仍为零任务，通知权限保留为允许；没有重新申请或修改通知权限。
- 安装位置的登录项注册成功，系统列表显示 Sidelet；注销成功并恢复为原来的关闭状态。未实际注销电脑测试登录后的自动启动。
- 前端 19 项测试、构建中的 Go 测试与 vet、4 项历史内存分析测试通过；进程诊断工具已对实际安装版进程读取验证。

更换应用位置时，先从旧版本关闭登录启动，退出旧版，再安装新位置并按需重新开启。注册成功不等于已经验证每种迁移或真实登录场景。

## 分发边界

当前仅为 ad-hoc 临时签名，没有 Developer ID 签名与公证。未修改 Gatekeeper、未删除隔离属性、未将本机成功启动当作从互联网下载后的安装验收。公开分发前需要开发者证书及公证流程，并补齐真实鼠标拖动、多屏 / Spaces、休眠和登录启动等验收。

Apple 参考：[打包 Mac 软件](https://developer.apple.com/documentation/xcode/packaging-mac-software-for-distribution)、[分发签名](https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac/)、[公证流程](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)。
