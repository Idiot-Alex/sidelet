# Sidelet 产品官网

独立的中文产品介绍与下载网站。使用 Svelte 5 / TypeScript / Vite，与桌面应用分别构建。

## 启动

在项目根目录：

```sh
npm --prefix website ci
npm run dev:website
```

访问 `http://127.0.0.1:5174`。不需要 Go、Wails 宿主或任务数据库。

## 构建与预览

```sh
npm run build:website
npm --prefix website run preview
```

静态产物位于 `website/dist/`。部署整个目录即可，不需要服务器 API 或 SPA 路由重写。网站支持放在域名根目录或子路径，资源和下载使用相对路径。域名确定后，应配置绝对 canonical 与 Open Graph 图片 URL；当前没有绑定域名、创建 GitHub Release 或公开部署。

## 下载入口

构建与开发启动会运行 `scripts/prepare-download.mjs`：

- 从 `packaging/macos/app.json` 读取版本和系统要求。
- 如 `build/releases/Sidelet-<版本>-local-arm64.dmg` 存在，将其复制到 `public/downloads/`，生成 SHA-256 校验文件和 `release.json`；生产构建将它们一起复制到 `dist/downloads/`。
- 如没有安装包，生成 `available: false` 的清单，网页显示“查看发布进度”。页面还检查文件存在性、类型和长度，避免显示指向缺失安装包的下载按钮。
- 更新桌面版本时先运行 `npm run package:macos`，再构建官网，以确保安装包与配置版本一致。

安装包与生成清单不进入 Git，因此干净检出也能构建，但默认没有 DMG 下载。公开发布前需把确认要发布的安装包放入上述路径并重新构建。当前 macOS 包是本地签名预览版，网站显示未公证状态与系统要求；Windows 不显示可用下载。

## 互动演示

`src/Demo.svelte` 复用桌面应用的 `EdgeStack` 和 `QuickCard`，提供折叠、悬停、打开、编辑 / 取消 / 保存、完成、稍后、重置及 Esc 关闭。所有数据只在页面内存中，不读写 SQLite，不访问本机任务，也不连接原生桥接操作。网页演示不代表原生鼠标穿透或焦点测试。

三套应用主题会同时更新侧栏和卡片。主题说明区的卡片是 `inert` 外观标本，不提供可点击的假操作。网站深浅外观独立于应用主题，初次遵从系统设置，按钮可手动切换。

`tests/responsive.html` 是开发服务器专用的真实 iframe 窄视口夹具（320 / 375 / 768 px），不作为生产入口打包。

## 验证

- Svelte 检查 0 错误 / 0 警告，生产构建通过。
- 浏览器真实鼠标悬停、键盘打开 / Esc 关闭与焦点返回、卡片编辑保存 / 取消 / 完成 / 稍后 / 重置通过。
- 三套应用主题、网站深浅外观、FAQ 展开检查通过。
- 窄视口夹具 320 / 375 / 768 px 无横向溢出；桌面首屏操作可见。
- 实际 HTTP 下载的 DMG 与已验证 macOS 包逐字节一致，字节数与 SHA-256 匹配。
- 无本地安装包的准备脚本与网页安装包缺失回退验证通过。

证据在 `build/results/website-v1/`，不进入 Git。窄视口验证不是移动设备实机验收；未运行 Lighthouse，不宣称 Core Web Vitals 已达标。没有新增分析追踪、账号或表单。

## 图像

两个配图由内置 imagegen 生成并转为 WebP，提示词与设计参数见 [设计与素材记录](DESIGN.md)。品牌图标复用 `assets/sidelet-icon.svg`。
