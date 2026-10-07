<script lang="ts">
  import { onMount } from 'svelte';
  import Demo from './Demo.svelte';
  import QuickCard from '../../frontend/src/components/QuickCard.svelte';
  import type { Todo } from '../../frontend/src/lib/model';

  const repo = 'https://github.com/Idiot-Alex/sidelet';
  let dark = $state(false);
  let theme = $state<'mac' | 'paper' | 'graphite'>('mac');
  let download = $state<'loading' | 'ready' | 'unavailable' | 'error'>('loading');
  let filename = $state('');
  let version = $state('');
  let bytes = $state(0);
  let hash = $state('');
  let minimumOS = $state('13.0');
  const base = import.meta.env.BASE_URL;
  const themeTodo: Todo = { id: 1, title: '让桌面也有自己的风格', description: '同一件小事，换一种喜欢的样子。', priority: 2, dueAt: 0, completed: false, completedAt: 0, snoozedUntil: 0 };
  const faqs = [
    ['现在可以用在哪些设备上？', '目前提供 macOS Apple Silicon（M 系列芯片）的本地预览版。Intel Mac 和 Windows 还未完成实机验收，暂不提供正式下载。'],
    ['我的任务会上传到服务器吗？', '桌面应用的任务保存在本机 SQLite 数据库中，目前没有账号系统或云同步。官网互动演示使用示例数据，刷新页面会重置，不会读取桌面应用里的任务。'],
    ['侧栏会挡住正在做的事吗？', '默认只留下细窄任务标记。悬停时展开任务内容，点击才显示操作卡片；标签之间和周围的空白没有背景层。原生穿透与多屏场景仍在持续验收。'],
    ['关闭主窗口，任务还在吗？', '关闭主窗口会隐藏窗口，Sidelet 继续在后台运行。可以从 Dock 或菜单栏重新打开；要结束应用，请选择退出。'],
    ['如何安装这个预览版？', '下载 DMG 并打开，把 Sidelet 拖入 Applications。当前为本地签名预览版，尚未经过 Apple 公证；首次打开可能需要在系统设置的“隐私与安全性”中确认。请仅安装你信任的来源。'],
  ];

  onMount(() => {
    const preference = matchMedia('(prefers-color-scheme: dark)');
    dark = preference.matches;
    const controller = new AbortController();
    let alive = true;
    async function loadRelease() {
      try {
        const response = await fetch(`${base}downloads/release.json`, { signal: controller.signal });
        if (!response.ok) throw new Error('manifest unavailable');
        const manifest = await response.json();
        if (!manifest.available) { if (alive) download = 'unavailable'; return; }
        if (typeof manifest.filename !== 'string' || !/^Sidelet-[\w.-]+-arm64\.dmg$/.test(manifest.filename)
          || manifest.url !== `downloads/${manifest.filename}` || !Number.isSafeInteger(manifest.bytes) || manifest.bytes <= 0
          || typeof manifest.sha256 !== 'string' || !/^[a-f0-9]{64}$/.test(manifest.sha256)) throw new Error('invalid manifest');
        const installer = await fetch(`${base}${manifest.url}`, { method: 'HEAD', signal: controller.signal });
        const contentLength = Number(installer.headers.get('content-length'));
        if (!installer.ok || installer.headers.get('content-type')?.includes('text/html') || (contentLength > 0 && contentLength !== manifest.bytes)) throw new Error('installer unavailable');
        if (!alive) return;
        filename = manifest.filename; version = String(manifest.version); bytes = manifest.bytes; hash = manifest.sha256; minimumOS = String(manifest.minimumOS);
        download = 'ready';
      } catch {
        if (alive) download = 'error';
      }
    }
    void loadRelease();
    return () => { alive = false; controller.abort(); };
  });
</script>

<svelte:head><meta name="theme-color" content={dark ? '#1d2520' : '#f6f7f3'} /></svelte:head>

<div class="site" id="top" data-color={dark ? 'dark' : 'light'}>
  <a class="skip-link" href="#main">跳到主要内容</a>
  <header class="site-header wrap">
    <a class="brand" href="#top" aria-label="Sidelet 首页"><img src="./favicon.svg" width="34" height="34" alt="" /><span>Sidelet</span></a>
    <nav aria-label="主导航"><a href="#experience">体验</a><a href="#details">细节</a><a href="#download">下载</a></nav>
    <button class="appearance-toggle" aria-label={dark ? '切换为浅色外观' : '切换为深色外观'} onclick={() => dark = !dark}>{dark ? '浅色' : '深色'}<span aria-hidden="true">◐</span></button>
  </header>

  <main id="main">
    <section class="hero wrap" aria-labelledby="hero-title">
      <div class="hero-copy">
        <p class="hero-label">轻轻停在桌面边缘的待办工具</p>
        <h1 id="hero-title">重要的事，<br /><span>在视线里。</span></h1>
        <p class="hero-description">悬停查看，点击处理。<br />留出空间，给正在做的事。</p>
        <div class="hero-actions"><a class="button primary" href="#download">获取 Mac 预览版<span aria-hidden="true">↗</span></a><a class="text-link" href="#experience">看看它如何工作<span aria-hidden="true">↓</span></a></div>
      </div>
      <div class="hero-demo"><Demo {theme} /></div>
    </section>

    <section class="experience wrap" id="experience" aria-labelledby="experience-title">
      <div class="section-heading"><h2 id="experience-title">从看见，到做完。<br /><span>刚刚好的存在感。</span></h2><p>不必一直开着任务列表。你需要多少信息，Sidelet 就展开多少。</p></div>
      <div class="experience-steps">
        <article><span class="step-mark" aria-hidden="true">│</span><h3>留在边缘</h3><p>几枚细窄标记，让重要任务有个位置。其余空间，留给你的桌面。</p></article>
        <article><span class="step-mark expanded-mark" aria-hidden="true">▱</span><h3>悬停，才展开</h3><p>鼠标经过时看清任务。移开后收起，不用来回切换窗口。</p></article>
        <article><span class="step-mark" aria-hidden="true">✓</span><h3>点击，就能处理</h3><p>完成、稍后、编辑，在浮动卡片里做完。保持手头的节奏。</p></article>
      </div>
    </section>

    <section class="details wrap" id="details" aria-labelledby="details-title">
      <img class="workspace-photo" src="./images/quiet-workspace.webp" width="1000" height="750" loading="lazy" alt="窗边的木质桌面上放着笔记本和一杯热饮，工作空间安静而简洁" />
      <div class="details-copy"><h2 id="details-title">少一点打扰，<br /><span>多一点从容。</span></h2><p class="section-intro">从快速记下，到暂时放下，每个小动作都尽量顺手。</p>
        <dl class="detail-list"><div><dt>灵感来了，先记下</dt><dd>全局快捷键呼出快速添加，支持简单时间识别。</dd></div><div><dt>忙的时候，先收起</dt><dd>稍后处理单条任务，或者用安静模式暂时隐藏侧栏。</dd></div><div><dt>按自己的习惯摆放</dt><dd>拖动把手调整整组位置，任务也可以重新排序。</dd></div><div><dt>资料留在自己手里</dt><dd>任务保存在本机，支持导出 JSON 和 CSV。</dd></div></dl>
      </div>
    </section>

    <section class="themes wrap" aria-labelledby="themes-title">
      <div class="theme-copy"><h2 id="themes-title">选一个，<br /><span>看着舒服的样子。</span></h2><p>精致 Mac、温暖纸色、深色石墨。任务页与浮动卡片，保持同一种风格。</p>
        <div class="theme-options" aria-label="应用主题预览">
          <button aria-pressed={theme === 'mac'} class:selected={theme === 'mac'} onclick={() => theme = 'mac'}><span class="swatch mac"></span>精致 Mac</button>
          <button aria-pressed={theme === 'paper'} class:selected={theme === 'paper'} onclick={() => theme = 'paper'}><span class="swatch paper"></span>温暖纸色</button>
          <button aria-pressed={theme === 'graphite'} class:selected={theme === 'graphite'} onclick={() => theme = 'graphite'}><span class="swatch graphite"></span>深色石墨</button>
        </div>
      </div>
      <div class="theme-preview" data-theme={theme}><div class="theme-card" inert aria-hidden="true"><QuickCard todo={themeTodo} editing={false} onCancel={() => {}} onClose={() => {}} onEdit={() => {}} onSave={() => {}} onComplete={() => {}} onSnooze={() => {}} onChoose={() => {}} /></div><p class="theme-caption" aria-live="polite">{theme === 'mac' ? '精致 Mac' : theme === 'paper' ? '温暖纸色' : '深色石墨'} · 卡片外观预览</p></div>
    </section>

    <section class="download-section" id="download" aria-labelledby="download-title">
      <div class="download-inner wrap">
        <img class="download-logo" src="./favicon.svg" width="72" height="72" alt="Sidelet 应用图标" />
        <h2 id="download-title">给重要的事，<br /><span>留一点位置。</span></h2>
        <p class="download-intro">从一件想做的小事开始。</p>
        <div class="download-action" aria-live="polite">
          {#if download === 'ready'}
            <a class="button primary" href={`${base}downloads/${filename}`} download={filename}>下载 macOS 预览版<span aria-hidden="true">↓</span></a>
            <p class="platform-note">Apple Silicon（M 系列） · macOS {minimumOS} 及以上</p>
            <p class="release-note">版本 {version}，{(bytes / 1024 / 1024).toFixed(1)} MB。本地签名预览版，尚未经过 Apple 公证。</p>
            <details class="checksum"><summary>查看安装包校验信息</summary><p>SHA-256</p><code>{hash}</code><a href={`${base}downloads/${filename}.sha256`} download>下载校验文件</a></details>
          {:else if download === 'loading'}
            <button class="button primary" disabled>正在检查安装包…</button><p class="platform-note">macOS Apple Silicon 预览版</p>
          {:else}
            <a class="button primary" href={`${repo}/releases`} target="_blank" rel="noreferrer">查看发布进度<span aria-hidden="true">↗</span></a><p class="platform-note">{download === 'error' ? '当前安装包暂不可用，请查看发布页。' : '此站点尚未附带安装包，发布后即可下载。'}</p>
          {/if}
        </div>
        <p class="windows-note">Windows 版本正在准备中。</p>
      </div>
    </section>

    <section class="faq wrap" aria-labelledby="faq-title"><h2 id="faq-title">还有一些，<br /><span>你可能想知道的。</span></h2><div class="faq-list">{#each faqs as [question, answer]}<details><summary>{question}<span aria-hidden="true">+</span></summary><p>{answer}</p></details>{/each}</div></section>
  </main>

  <footer class="site-footer wrap"><a class="brand" href="#top"><img src="./favicon.svg" width="28" height="28" alt="" /><span>Sidelet</span></a><p>让任务在视线里，让注意力留在当下。</p><div><a href={repo} target="_blank" rel="noreferrer">GitHub</a><a href={`${repo}/issues`} target="_blank" rel="noreferrer">反馈问题<span aria-hidden="true">↗</span></a></div></footer>
</div>
