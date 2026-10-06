<script lang="ts">
  import AppHeader from './AppHeader.svelte';
  import Icon from './Icon.svelte';
  import { request, send, hostPlatform, type NotificationStatus } from '../lib/bridge';
  import { themes, loginLabels, notificationLabels, type SettingsState, type Preferences } from '../lib/settings';
  let { settings, notificationStatus, externalError, onClose }: { settings?: SettingsState; externalError: string; notificationStatus: NotificationStatus; onClose: () => void } = $props();
  let draft = $state<Preferences>({ version: 1, edge: { defaultSide: 'right', defaultDensity: 'normal' }, startup: { enabled: false, showMainWindow: false }, appearance: { theme: 'mac', showDockIcon: true } });
  let busy = $state(false);
 let loginChecked = $state(false);
  let error = $state('');
  let feedback = $state('');
  let exporting = $state(false);
  let exportFeedback = $state('');
  let exportError = $state('');
  async function exportTasks(format: 'json' | 'csv') {
    if (exporting || busy || !settings) return;
    exporting = true; exportFeedback = ''; exportError = '';
    try {
      const result = await request('export', { format });
      exportFeedback = result.cancelled ? '已取消导出' : `已导出 ${result.count ?? 0} 项任务到 ${result.filename ?? ''}`;
    } catch (cause) {
      exportError = cause instanceof Error ? cause.message : '导出失败，请重试。';
    } finally { exporting = false; }
  }
  $effect(() => { if (settings) { draft = { ...settings.value, edge: { ...settings.value.edge }, startup: { ...settings.value.startup }, appearance: { ...settings.value.appearance } }; loginChecked = settings.loginStatus === "enabled" || settings.loginStatus === "requiresApproval"; } });
  async function save(login?: boolean) {
    if (!settings || busy) return;
    busy = true; error = ''; feedback = '';
    try {
      await request(login === undefined ? 'settings-save' : 'settings-login', login === undefined ? { settings: draft } : { enabled: login });
      feedback = '已保存';
    } catch (cause) {
      error = cause instanceof Error ? cause.message : '保存失败，请重试。';
      draft = { ...settings.value, edge: { ...settings.value.edge }, startup: { ...settings.value.startup }, appearance: { ...settings.value.appearance } };
    } finally { loginChecked = settings.loginStatus === "enabled" || settings.loginStatus === "requiresApproval"; busy = false; }
  }
</script>

<main class="settings" data-sidelet>
  <AppHeader><button class="back-button" disabled={busy || exporting} onclick={onClose}><Icon name="back" />返回我的任务</button></AppHeader>
  <div class="settings-content">
  <div class="intro"><div><p class="eyebrow">为你的习惯，留一点空间</p><h1>设置</h1><p>界面与桌面，按照你喜欢的方式。</p></div><span class="save-status" role="status">{busy ? '正在保存…' : feedback || (!settings ? '正在读取…' : '更改自动保存')}</span></div>
  {#if error || settings?.error || externalError}<p class="error-message" role="alert">{error || settings?.error || externalError}</p>{/if}
  <fieldset disabled={busy || exporting || !settings || !!settings.error}>
    <section aria-labelledby="appearance-heading">
      <div class="section-heading"><h2 id="appearance-heading">外观</h2><p>同一种风格，贯穿任务窗口与桌面卡片。</p></div>
      <div class="theme-options" role="group" aria-label="界面主题">
        {#each themes as theme}
          <button class="theme-option" class:selected={draft.appearance.theme === theme.id} aria-label={theme.name} aria-pressed={draft.appearance.theme === theme.id} onclick={() => { draft.appearance.theme = theme.id; void save(); }}>
            <div class="theme-preview" data-theme={theme.id} aria-hidden="true"><div class="preview-bar"><i></i><i></i><i></i></div><div class="preview-content"><div class="preview-tasks"><b></b><span></span><span></span><span></span></div><div class="preview-aside"><i></i><i></i><b></b></div></div><span class="preview-type">Aa</span></div>
            <div class="theme-name"><strong>{theme.name}</strong><span class="selection-mark">{#if draft.appearance.theme === theme.id}<Icon name="check" size={12} />{/if}</span></div><p>{theme.description}</p>
          </button>
        {/each}
      </div>
      {#if hostPlatform === 'darwin'}<div class="row dock-row"><div><label for="show-dock">在 Dock 中显示</label><p>点击图标打开任务窗口。关闭后仍可从顶部菜单栏进入。</p></div><input id="show-dock" type="checkbox" bind:checked={draft.appearance.showDockIcon} onchange={event => { draft.appearance.showDockIcon = event.currentTarget.checked; void save(); }} /></div>{/if}
    </section>
    <section aria-labelledby="startup-heading">
      <h2 id="startup-heading">启动行为</h2>
      <div class="row"><div><label for="login">登录 Mac 后启动 Sidelet</label><p>自动在后台运行，让桌面任务和提醒保持可用。</p><small>系统状态：{settings ? loginLabels[settings.loginStatus] ?? '状态未知' : '读取中…'}</small></div><input id="login" aria-label="登录后启动 Sidelet" type="checkbox" bind:checked={loginChecked} disabled={!settings?.loginAvailable} onchange={event => save(event.currentTarget.checked)} /></div>
      {#if settings && !settings.loginAvailable && hostPlatform === 'darwin'}<p class="hint">测试资料目录不更改系统登录项，请在正常版本中设置。</p>{/if}
      {#if hostPlatform === 'darwin'}<div class="actions"><button onclick={() => send('open-login-settings')}>打开系统登录项</button><button onclick={() => send('settings-refresh')}>检查登录状态</button></div>{/if}
      {#if settings?.loginStatus === 'requiresApproval'}<p class="hint">请在系统登录项中允许 Sidelet；完成后返回这里检查状态。</p>{/if}
      <div class="row"><div><label for="show-main">启动时显示主窗口</label><p>关闭后保留菜单栏入口与已固定的桌面任务。</p></div><input id="show-main" type="checkbox" bind:checked={draft.startup.showMainWindow} onchange={event => { draft.startup.showMainWindow = event.currentTarget.checked; void save(); }} /></div>
    </section>
    <section aria-labelledby="edge-heading">
      <h2 id="edge-heading">新任务组默认值</h2><p class="hint">仅在创建新任务组时使用。现有任务组的位置与密度可在“我的任务”中调整。</p>
      <div class="row"><label for="default-side">默认屏幕边缘</label><select id="default-side" bind:value={draft.edge.defaultSide} onchange={event => { draft.edge.defaultSide = event.currentTarget.value as "left" | "right"; void save(); }}><option value="left">左侧</option><option value="right">右侧</option></select></div>
      <div class="row"><label for="default-density">默认标签密度</label><select id="default-density" bind:value={draft.edge.defaultDensity} onchange={event => { draft.edge.defaultDensity = event.currentTarget.value as Preferences["edge"]["defaultDensity"]; void save(); }}><option value="compact">紧凑</option><option value="normal">标准</option><option value="relaxed">宽松</option></select></div>
    </section>
  </fieldset>
  <section aria-labelledby="notification-heading">
    <h2 id="notification-heading">系统通知 <span class="status">{notificationLabels[notificationStatus.authorization] ?? '读取中…'}</span></h2>
    <p class="hint">仅勾选“提醒我”的任务会发送通知，Sidelet 需在后台运行。</p>
    {#if notificationStatus.message}<p class="hint">{notificationStatus.message}</p>{/if}
    {#if hostPlatform === 'darwin'}<div class="actions"><button onclick={() => send('notification-permission')}>{notificationStatus.authorization === 'notDetermined' ? '开启系统通知' : '检查通知权限'}</button><button onclick={() => send('open-notification-settings')}>打开系统通知设置</button></div><p class="hint">在系统通知列表中选择 Sidelet，调整横幅和声音。</p>{/if}
  </section>
  <section aria-labelledby="quick-add-heading">
    <h2 id="quick-add-heading">快速添加</h2>
    <p class="hint">在任何应用中按 Control + Shift + Space，记下任务。Enter 保存，Esc 取消并返回之前的应用。</p>
    <p class="hint">支持“明天下午3点 联系客户”等简单时间，默认不固定到桌面、不发送系统通知。</p>
    {#if settings?.quickAddShortcutError}<p class="error-message" role="alert">{settings.quickAddShortcutError}</p>{/if}
    <div class="actions"><button onclick={() => send('quick-add-open')}>打开快速添加</button></div>
  </section>
  <section aria-labelledby="export-heading">
    <h2 id="export-heading">导出任务</h2>
    <p class="hint">导出全部任务，包含已完成、未固定和暂时隐藏的任务。JSON 保留完整任务字段与桌面布局，CSV 适合用表格查看。</p>
    <div class="actions"><button class="export-button" disabled={busy || exporting || !settings} onclick={() => exportTasks('json')}><Icon name="download" />导出 JSON</button><button class="export-button" disabled={busy || exporting || !settings} onclick={() => exportTasks('csv')}><Icon name="download" />导出 CSV</button></div>
    <p class="hint">导出的是点击时已保存的数据，不包含未提交的草稿。当前版本暂不支持导入。</p>
    {#if exporting || exportFeedback}<p class="export-feedback" role="status">{exporting ? '请选择保存位置…' : exportFeedback}</p>{/if}
    {#if exportError}<p class="error-message" role="alert">{exportError}</p>{/if}
  </section>
<footer>Sidelet {settings?.appVersion ?? "…"} · 构建 {settings?.appBuild ?? "…"} · 本地预览版</footer>
</div>
</main>

<style>
  .settings { min-height:100vh; padding:0 32px 36px; background:var(--app-bg); color:var(--ink); }
  .settings-content { max-width:850px; margin:0 auto; }
  .intro { display:flex; justify-content:space-between; align-items:flex-end; padding:30px 0 24px; margin:0; }
  .eyebrow { font:11px var(--font-ui); color:var(--muted); letter-spacing:.05em; margin:0 0 9px; }
  h1 { font-family:var(--font-heading); font-size:30px; font-weight:var(--heading-weight); line-height:1.25; margin:0 0 10px; letter-spacing:-.035em; }
  .intro p:not(.eyebrow) { color:var(--muted); font-size:13px; }
  p { line-height:1.6; color:var(--muted); font-size:12px; margin:5px 0; }
  fieldset { border:0; padding:0; margin:0; min-width:0; }
  section { border:1px solid var(--line); border-radius:var(--radius); background:var(--surface); padding:22px 24px; margin-bottom:16px; }
  h2 { font-family:var(--font-heading); font-size:16px; margin:0 0 14px; font-weight:600; }
  .section-heading h2 { margin-bottom:5px; }.section-heading { margin-bottom:18px; }
  .row { display:flex; justify-content:space-between; align-items:center; gap:28px; padding:15px 0; }.row:last-child { padding-bottom:0; }.row + .row { border-top:1px solid var(--line); margin-top:8px; }
  .row label { font-size:13px; font-weight:500; }.row p { margin:5px 0; }small { font-size:11px; color:var(--muted); }
  .dock-row { border-top:1px solid var(--line); margin-top:20px; padding-top:20px; }
  input[type=checkbox] { appearance:none; -webkit-appearance:none; width:32px; height:19px; border-radius:20px; background:var(--surface-alt); border:1px solid var(--line); flex-shrink:0; position:relative; cursor:pointer; margin:0; }
  input[type=checkbox]::after { content:''; position:absolute; top:2px; left:2px; width:13px; height:13px; border-radius:50%; background:var(--subtle); }
  input[type=checkbox]:checked { background:var(--accent); border-color:var(--accent); }input[type=checkbox]:checked::after { left:15px; background:var(--on-accent); }input:disabled { opacity:.45; cursor:default; }
  select,button { font:12px var(--font-ui); border:1px solid var(--line); background:var(--field); color:var(--ink); border-radius:var(--control-radius); padding:8px 12px; }
  select { min-width:128px; }button { cursor:pointer; }button:hover:not(:disabled) { background:var(--surface-alt); }
  button:disabled { opacity:.5; cursor:default; }fieldset:disabled { opacity:.7; }
  .back-button { display:flex; gap:7px; align-items:center; border-color:transparent; background:transparent; color:var(--muted); }
  .actions { display:flex; gap:8px; flex-wrap:wrap; margin:12px 0 0; }.hint { max-width:650px; }.save-status { font-size:11px; color:var(--subtle); padding-bottom:2px; }
  .export-button { display:flex; align-items:center; gap:7px; }.export-feedback { color:var(--accent); overflow-wrap:anywhere; margin-top:12px; }
  .status { display:inline-block; font:11px var(--font-ui); margin-left:8px; color:var(--accent); background:var(--accent-soft); padding:4px 7px; border-radius:5px; }
  .error-message { padding:10px 12px; background:var(--danger-soft); color:var(--danger); border-radius:var(--control-radius); }
  footer { padding:6px 0 0; text-align:center; color:var(--subtle); font-size:11px; }
  .theme-options { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:16px; }
  .theme-option { text-align:left; padding:0; border:0; background:transparent; border-radius:8px; }
  .theme-option:hover:not(:disabled) { background:transparent; }
  .theme-preview { position:relative; height:118px; border:1px solid var(--line); border-radius:8px; background:var(--app-bg); overflow:hidden; padding:0 12px; }
  .theme-option.selected .theme-preview { outline:2px solid var(--accent); outline-offset:3px; }
  .theme-option:hover:not(:disabled) .theme-preview { opacity:.85; }
  .preview-bar { display:flex; gap:3px; height:24px; align-items:center; border-bottom:1px solid var(--line); }.preview-bar i { width:4px; height:4px; background:var(--subtle); opacity:.55; border-radius:50%; }
  .preview-content { display:flex; gap:10px; padding-top:12px; }.preview-tasks { flex:1; }.preview-tasks b { display:block; width:40%; height:5px; background:var(--ink); border-radius:2px; margin-bottom:9px; }.preview-tasks span { display:block; height:13px; background:var(--surface); margin:4px 0; border:1px solid var(--line); border-radius:3px; }.preview-tasks span::before { content:''; display:block; width:4px; height:4px; border:1px solid var(--accent); border-radius:50%; margin:3px 5px; }
  .preview-aside { width:30%; background:var(--surface); border:1px solid var(--line); border-radius:4px; padding:7px 5px; }.preview-aside i { display:block; height:6px; margin-bottom:5px; background:var(--surface-alt); border-radius:1px; }.preview-aside b { display:block; background:var(--accent); height:7px; border-radius:2px; margin-top:9px; }
  .preview-type { position:absolute; right:15px; top:3px; font:14px var(--font-heading); color:var(--ink); }
  .theme-name { display:flex; align-items:center; justify-content:space-between; margin-top:13px; }.theme-name strong { font-size:13px; font-weight:550; }.selection-mark { display:grid; place-items:center; border:1px solid var(--line); width:17px; height:17px; border-radius:50%; }.selected .selection-mark { background:var(--accent); border-color:var(--accent); color:var(--on-accent); }.theme-option p { margin-top:4px; font-size:11px; }
  @media(max-width:900px) { .settings { padding:0 24px 28px; }section { padding:20px; }.theme-options { gap:14px; } }
  select { appearance:none; -webkit-appearance:none; min-height:36px; padding-right:30px; background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='m3 4.5 3 3 3-3' fill='none' stroke='%23839088' stroke-width='1.4' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E"); background-position:right 10px center; background-repeat:no-repeat; }
</style>
