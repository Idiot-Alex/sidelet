<script lang="ts">
  import { tick } from 'svelte';
  import Icon from './Icon.svelte';
  import { request, send, type QuickAddState, type ParsedTask } from '../lib/bridge';
  let { session }: { session: QuickAddState } = $props();
  let input = $state('');
  let pin = $state(false);
  let recognize = $state(true);
  let busy = $state(false);
  let error = $state('');
  let previewError = $state('');
  let preview = $state<ParsedTask>();
  let field = $state<HTMLInputElement>();
  let resetVersion = -1;
  let openRevision = -1;
  $effect(() => {
    if (session.resetVersion !== resetVersion) {
      resetVersion = session.resetVersion; input = ''; pin = false; recognize = true; error = ''; previewError = ''; preview = undefined;
    }
    if (session.open && session.revision !== openRevision) {
      openRevision = session.revision; void tick().then(() => { field?.focus(); field?.setSelectionRange(field.value.length, field.value.length); });
    }
  });
  $effect(() => {
    const text = input, enabled = recognize, revision = session.revision;
    if (!session.open || busy || session.saving || !text.trim()) { preview = undefined; previewError = ''; return; }
    let cancelled = false;
    const timer = setTimeout(() => {
      void request('quick-add-preview', { action: { title: text }, enabled, revision }).then(result => {
        if (!cancelled) { preview = result.parsed; previewError = ''; }
      }).catch(cause => { if (!cancelled) { preview = undefined; previewError = cause instanceof Error ? cause.message : '无法识别时间。'; } });
    }, 150);
    return () => { cancelled = true; clearTimeout(timer); };
  });
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy || session.saving || !session.open || !input.trim()) return;
    busy = true; error = '';
    try { await request('quick-add-save', { action: { title: input, pin }, enabled: recognize, revision: session.revision }); }
    catch (cause) { error = cause instanceof Error ? cause.message : '保存失败，输入已保留，请重试。'; }
    finally {
      busy = false;
      if (error && session.open) {
        await tick();
        if (session.open) field?.focus();
      }
    }
  }
  function cancel() { if (!busy && !session.saving) send('quick-add-cancel', { revision:session.revision }); }
  function key(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) { if (event.key === 'Enter') event.preventDefault(); return; }
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); cancel(); }
    if (event.key === 'Enter' && (busy || session.saving)) event.preventDefault();
  }
</script>

<div class="quick-add" role="dialog" aria-label="快速添加任务" tabindex="-1" data-sidelet data-hit onkeydown={key}>
  <form onsubmit={submit}>
    <header><span class="brand"><Icon name="plus" size={17} />快速添加</span><button type="button" class="close" aria-label="取消快速添加" disabled={busy || session.saving} onclick={cancel}><Icon name="close" size={16} /></button></header>
    <label class="input-label" for="quick-add-title">任务标题</label>
    <input id="quick-add-title" aria-label="快速添加任务标题" bind:this={field} bind:value={input} oninput={() => error = ''} maxlength="600" autocomplete="off" placeholder="明天下午3点 联系客户" disabled={busy || session.saving} />
    <div class="preview" aria-live="polite">
      {#if error || previewError}<p class="error" role="alert">{error || previewError}</p>
      {:else if preview?.dueAt}<span class="date">{new Date(preview.dueAt).toLocaleString('zh-CN', { month:'numeric', day:'numeric', hour:'2-digit', minute:'2-digit', hour12:false })}</span><span class="parsed-title">{preview.title}</span>
      {:else}<span class="hint">{recognize ? '支持今天、明天、后天，如“明天15:30 联系客户”。' : '整句保存为标题，不设置截止时间。'}</span>{/if}
    </div>
    <footer><div class="options"><label><input type="checkbox" bind:checked={recognize} disabled={busy || session.saving} />识别时间</label><label><input type="checkbox" bind:checked={pin} disabled={busy || session.saving} />固定到桌面</label></div><button class="save" type="submit" disabled={busy || session.saving || !session.open || !input.trim() || !!previewError}>{busy || session.saving ? '保存中…' : '添加'}<kbd>↵</kbd></button></footer>
    <p class="key-hint">Enter 保存 · Esc 取消 · 截止时间默认不发送通知</p>
  </form>
</div>

<style>
  .quick-add { width:100vw; height:100vh; padding:12px; color:var(--ink); font-family:var(--font-ui); outline:none; }
  form { height:100%; display:flex; flex-direction:column; border:1px solid var(--line); border-radius:var(--radius); background:var(--surface); padding:15px 18px 12px; overflow:auto; box-shadow:0 4px 18px rgb(0 0 0 / .08); }
  header,footer { display:flex; align-items:center; justify-content:space-between; gap:12px; }header { margin-bottom:12px; }.brand { display:flex; align-items:center; gap:7px; font-size:12px; font-weight:600; color:var(--muted); }
  .close { display:grid; place-items:center; padding:3px; border:0; background:transparent; color:var(--muted); border-radius:4px; }
  .input-label { position:absolute; width:1px; height:1px; overflow:hidden; clip-path:inset(50%); }
  input:not([type]) { width:100%; min-height:42px; padding:8px 0; border:0; border-bottom:1px solid var(--line); border-radius:0; background:transparent; font-size:19px; color:var(--ink); outline:none; }
  input::placeholder { color:var(--subtle); }input:focus-visible { outline:none; border-bottom-color:var(--accent); }
  .preview { display:flex; gap:8px; align-items:center; min-height:37px; padding:7px 0; font-size:11px; overflow-wrap:anywhere; }.date { flex-shrink:0; color:var(--accent); background:var(--accent-soft); padding:3px 6px; border-radius:4px; }.parsed-title { overflow:hidden; white-space:nowrap; text-overflow:ellipsis; }.hint { color:var(--muted); }.error { margin:0; color:var(--danger); }
  footer { margin-top:auto; }.options { display:flex; flex-wrap:wrap; gap:14px; font-size:11px; color:var(--muted); }.options label { display:flex; gap:5px; align-items:center; }.options input { accent-color:var(--accent); margin:0; }
  .save { display:flex; align-items:center; gap:12px; padding:8px 12px; border:0; background:var(--accent); color:var(--on-accent); border-radius:var(--control-radius); font-size:12px; }kbd { font:12px var(--font-ui); opacity:.7; }
  .key-hint { margin:8px 0 0; font-size:10px; color:var(--subtle); }
</style>
