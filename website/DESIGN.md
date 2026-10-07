# 官网设计与素材记录

## 设计方向

中文桌面工具官网，面向希望减少任务切换的个人用户。保留应用灰绿品牌色和系统字体，采用原生 CSS 的轻量 Mac 工具审美，不声称使用某个官方设计系统。

- DESIGN_VARIANCE: 5
- MOTION_INTENSITY: 3
- VISUAL_DENSITY: 3

单一页面主题，深浅外观各用一套站点变量；应用主题标本保持产品实际配色。首页左右布局，中段用开放的信息排列、照片与定义列表、可切换卡片标本、集中下载区、FAQ，避免重复的卡片网格。动画仅用于状态反馈，不设循环、滚动监听或无目的自动动画。减弱动态偏好会禁用过渡和顺滑滚动。

字体使用 macOS 与中文系统字体，无远程字体请求。品牌图标直接复用原有 SVG；应用内部图标复用既有组件。桌面背景上运行实际产品组件，配图不伪造应用界面。

## 生成素材

生成方式：内置 imagegen。图片经过尺寸调整并压缩为 WebP，默认生成的源图仍保留；项目消费的是以下文件。

### `public/images/desktop-lake.webp`

1440 × 901，约 179 KiB。作为真实侧栏互动演示背景，首屏预加载。

最终提示词：

> Create a single landscape 16:10 photographic desktop wallpaper asset for the Sidelet task app product website. Photorealistic aerial landscape: a pale silver-green alpine lake in the lower right curving around an uninhabited rocky shoreline, soft sage hills fading into fine mist in the upper left, cool morning light, soft natural film texture, restrained quiet color palette of mineral gray, muted sage and off-white. Calm, refined contemporary Mac wallpaper aesthetic. Broad uncluttered negative space, detailed but low contrast so actual live task UI can be overlaid by the website. Edge-to-edge landscape only. Absolutely no device, no screen frame, no UI, no text, no logos, no people. This is the backdrop for a real interactive app component, not a mockup screenshot.

### `public/images/quiet-workspace.webp`

1000 × 750，约 66 KiB。功能说明旁的窗边工作空间照片，延迟加载。

最终提示词：

> Create one photorealistic landscape 4:3 editorial photograph for Sidelet, a calm minimal desktop task app website. A real quiet home working desk next to a large window, light mineral-gray oak desktop, a slim closed silver laptop at the bottom left, an off-white notebook with no readable writing, a small ceramic cup, sheer white curtain with gentle morning side light, a single muted sage wall in the background. Close composed crop, mostly desk and negative space, no people, authentic subtle imperfections and natural material texture, no glossy staged CGI. Restrained neutral color palette with only muted gray-green accents. No screen interface, no text, no logos, no labels, no overlays, no watermark. This is a supporting lifestyle image, not a UI mockup.
