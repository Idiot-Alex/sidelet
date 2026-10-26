# macOS 打包与官网下载校验

2026-10-26，0.2.0 / build 46。最近两次 UI 提交仍沿用了 build 45；本轮统一提升构建号，并修复官网可能把旧 DMG 标为当前构建的问题。

## 生成安装包

```sh
bash scripts/package-macos.sh arm64
npm run build:website
```

输出位于 `build/releases/`：

- `Sidelet-0.2.0-build46-local-arm64.dmg`
- 同名 `.dmg.sha256` 校验文件
- 同名 `.dmg.json` 核验记录

构建元数据仍统一来自 `packaging/macos/app.json`，Windows 发布工具也读取此配置。macOS 构建将版本、构建号、架构、Git 提交及工作区是否有改动写入应用资源 `build-info.json`，再签名整个 bundle。`sourceDirty: true` 表示来源包含未提交改动，不能仅用提交号声称产物来自干净源码。

打包脚本在临时目录完成 DMG 后，先执行 `hdiutil verify`，再只读挂载，检查实际包内应用的版本、构建号、bundle ID、最低系统版本、可执行文件名、构建记录、签名及 Mach-O 架构。核验结束卸载，再记录安装包大小、SHA-256、签名后的二进制 SHA-256。成功后才把 DMG 和核验记录移入产物目录。

可独立重新核验已有包：

```sh
python3 scripts/macos-package.py verify build/releases/Sidelet-0.2.0-build46-local-arm64.dmg arm64
```

## 官网的下载判定

官网使用包含构建号的精确文件名。没有当前构建包时正常生成无下载状态，不会回退到没有构建号的旧 DMG。当前包存在时，必须同时提供打包阶段生成的 `.dmg.json`，并验证版本、构建号、平台、架构、应用标识、最低系统版本、字节数和 SHA-256。

缺少 / 损坏记录、重命名旧包、替换文件、哈希不符都会使官网构建失败。校验开始时先清除下载可用状态及生成目录中的旧 Sidelet 安装包，防止残留下载继续被展示。核验成功后写入已检查的原始字节，最后发布 `release.json`。网页同时检查文件名与版本 / 构建号一致，并在下载说明中显示 build 号。

官网本身不需要在 macOS 挂载 DMG；其他构建系统可使用由 macOS 核验生成的包与记录。记录用于防止误配和陈旧产物，不是额外的发行签名；本地包仍为 ad-hoc 签名、未经过 Apple 公证。

## 自动检查

```sh
python3 -B scripts/test-macos-package.py
npm run test:website
```

Python 检查覆盖包内旧版本、构建记录不一致、架构不符、签名失败，以及验证异常后卸载挂载点。Node 检查覆盖有效包发布、只有旧包时禁用下载、版本 / 构建 / 架构 / 应用 / 最低系统版本不匹配、同大小内容替换、截断和记录缺失 / 损坏。macOS 打包和官网构建分别自动执行这些检查。

本轮不安装或替换日常应用，不触碰任务资料，也不自动发布官网或 GitHub Release。Windows 实机透明、鼠标交接与输入法验收仍需后续完成。

## 本轮结果

- Python 5 项、Node 11 项校验测试通过；macOS 打包中的 Go 应用 / internal 测试与 vet 通过，官网检查 0 错误、0 警告，Vite 构建成功。
- 实际生成并只读挂载核验 ARM64 build 46。DMG 大小 6,580,496 字节，SHA-256 为 `df3c1a94340de54c62065ffe7149a3362772dc575334cba59fac1de8daa8362b`。
- 实际复制 10 月 17 日的旧 DMG，重命名为 build 46 文件名后验证：包内元数据不符被拒绝，挂载点正常卸载，没有生成核验记录。
- 当前包尚未生成时，官网构建正确产出 `available: false`；包核验完成后再次构建，`public/downloads` 和 `dist/downloads` 的文件哈希均与核验包一致，旧无构建号下载文件已清除。
- Chrome 本地预览确认下载入口指向 `Sidelet-0.2.0-build46-local-arm64.dmg`，页面显示 `0.2.0（build 46）` 和 6.3 MB。检查后已关闭临时预览页面和服务。

构建来自 `0d4f5a7` 加本轮未提交改动，包内 `sourceDirty` 如实为 `true`。本地摘要和日志位于 `build/results/package46-20261026/`。
