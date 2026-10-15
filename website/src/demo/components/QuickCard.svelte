<script lang="ts">
  import Icon from './Icon.svelte';
  import IconText from './IconText.svelte';
  import { onMount, tick } from 'svelte';
  import { trackMemory } from '../lib/memory';
  onMount(() => trackMemory('QuickCard'));
  import { dueLabel, type Todo } from '../lib/model';
  import { normalizePriority, priorityLabel } from '../lib/priority';
  import { measureQuickCard } from '../lib/quickCardSize';
  let { todo, overflow = [], editing, busy = false, error = '', now = Date.now(), onClose, onEdit, onSave, onComplete, onSnooze, onChoose, onCancel, onPresence, onSize }:
    { todo?: Todo; overflow?: Todo[]; editing: boolean; busy?: boolean; error?: string; now?: number; onClose: () => void; onEdit: () => void; onSave: (title: string, description: string) => void; onComplete: () => void; onSnooze: (duration: string) => void; onChoose: (todo: Todo) => void; onCancel: () => void; onPresence?: (inside: boolean) => void; onSize?: (height: number) => void } = $props();
  let title = $state('');
  let description = $state('');
  let snooze = $state(false);
  let input = $state<HTMLInputElement>();
  let card = $state<HTMLDivElement>();
  let disposed = false;
  let lastHeight = 0;
  function reportSize() {
    if (disposed) return;
    const height = measureQuickCard();
    if (height !== lastHeight) { lastHeight = height; onSize?.(height); }
  }
  onMount(() => {
    const observer = new ResizeObserver(reportSize);
    const release = trackMemory('ResizeObserver');
    if (card) observer.observe(card);
    reportSize();
    return () => { disposed = true; observer.disconnect(); release(); };
  });
  $effect(() => {
    todo?.title; todo?.description; todo?.dueAt; todo?.priority; overflow; editing; snooze; error;
    void tick().then(reportSize);
  });
  $effect(() => { todo?.id; editing; overflow.length; snooze = false; });
  const priority = $derived(normalizePriority(todo?.priority));
  let loadedID = 0;
  let wasEditing = false;
  $effect(() => {
    if (editing && todo && (!wasEditing || loadedID !== todo.id)) {
      loadedID = todo.id;
      title = todo.title; description = todo.description;
      void tick().then(() => { input?.focus(); input?.select(); });
    }
    if (!editing && wasEditing) void tick().then(() => card?.focus({ preventScroll: true }));
    wasEditing = editing;
  });
  function key(event: KeyboardEvent) {
    if (!editing) return;
    if (event.isComposing) return;
    if (busy) { if (event.key === 'Escape' || event.key === 'Enter') { event.preventDefault(); event.stopPropagation(); } return; }
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onCancel(); }
    if (event.key === 'Enter' && (event.target instanceof HTMLInputElement || event.ctrlKey || event.metaKey)) {
      event.preventDefault(); event.stopPropagation(); if (title.trim()) onSave(title, description);
    }
  }
</script>

<div bind:this={card} class="quick-card" class:editing role="dialog" tabindex="-1" aria-label="任务快速操作" data-sidelet data-hit onkeydown={key} onpointerenter={() => onPresence?.(true)} onpointerleave={() => onPresence?.(false)}>
  <header><span class="eyebrow">{overflow.length ? '更多任务' : editing ? '快速编辑' : '当前任务'}</span><button class="close" aria-label="关闭快速卡片" onclick={onClose} disabled={busy}><Icon name="close" size={16} /></button></header>
  {#if overflow.length}
    <div class="quick-body overflow-list">
      {#each overflow as item (item.id)}<button onclick={() => onChoose(item)}><span>{item.title}</span><Icon name="chevron" size={14} /></button>{/each}
    </div>
  {:else if todo}
    <div class="quick-body">
      {#if editing}
        <label class="field">任务标题<input bind:this={input} bind:value={title} maxlength="500" disabled={busy} /></label>
        <label class="field">备注<textarea bind:value={description} rows="5" disabled={busy}></textarea></label>
      {:else}
        <h2>{todo.title}</h2>
        {#if priority || todo.dueAt}
          <div class="metadata">
            {#if priority}<span class="priority" class:urgent={priority === 3}>{priorityLabel(priority)}</span>{/if}
            {#if todo.dueAt}<span class="due" class:overdue={todo.dueAt <= now}><IconText name="clock" size={12} gap={4}>{dueLabel(todo, now) ? dueLabel(todo, now) + ' · ' : ''}{new Date(todo.dueAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })}</IconText></span>{/if}
          </div>
        {/if}
        {#if todo.description.trim()}<p class="description">{todo.description}</p>{/if}
      {/if}
    </div>
    {#if error}<p class="save-error" role="alert">{error}</p>{/if}
    <footer class:editing class:snoozing={snooze && !editing}>
      {#if editing}
        <button class="primary" onclick={() => onSave(title, description)} disabled={busy || !title.trim()}><IconText name="check" size={14} gap={5}>{busy ? '保存中…' : '保存'}</IconText></button><button onclick={onCancel} disabled={busy}>取消</button>
      {:else if snooze}
        <button disabled={busy} onclick={() => onSnooze('30m')}>30 分钟</button><button disabled={busy} onclick={() => onSnooze('1h')}>1 小时</button><button disabled={busy} onclick={() => onSnooze('tomorrow')}>明天 09:00</button><button class="return" disabled={busy} aria-label="返回操作" onclick={() => snooze = false}><IconText name="back" size={13} gap={4}>返回</IconText></button>
      {:else}
        <button class="primary" onclick={onComplete} disabled={busy}><IconText name="check" size={14} gap={5}>完成</IconText></button><button onclick={() => snooze = true} disabled={busy}><IconText name="clock" size={14} gap={5}>稍后</IconText></button><button onclick={onEdit} disabled={busy}><IconText name="edit" size={14} gap={5}>编辑</IconText></button>
      {/if}
    </footer>
  {:else}
    <div class="quick-body"><p>选择一条任务开始操作。</p></div>
  {/if}
</div>

<style>
  .quick-card { min-height: min(160px, 100vh); max-height: min(390px, 100vh); display: flex; flex-direction: column; background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); overflow: hidden; }
  .quick-card.editing { height: 100%; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 10px 16px 2px; flex-shrink: 0; }
  .eyebrow { font: 11px var(--font-ui); letter-spacing: .03em; color: var(--muted); }
  .close { padding: 0; border: 0; background: transparent; font-size: 22px; color: var(--muted); }
  .quick-body { flex: 0 1 auto; min-height: 0; overflow-y: auto; padding: 10px 16px 16px; }
  .editing .quick-body { flex: 1; }
  h2 { font-family: var(--font-heading); font-size: 18px; line-height: 1.4; font-weight: var(--heading-weight); margin: 0; overflow-wrap: anywhere; }
  .metadata { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; margin-top: 10px; }
  .priority { font-size: 10px; line-height: 18px; padding: 0 6px; border-radius: 4px; color: var(--warning); background: var(--warning-soft); }
  .priority.urgent { color: var(--danger); background: var(--danger-soft); }
  .due { font-size: 11px; line-height: 18px; color: var(--muted); }
  .due.overdue { color: var(--danger); }
  .description { margin: 12px 0 0; white-space: pre-wrap; font-size: 13px; line-height: 1.65; color: var(--muted); overflow-wrap: anywhere; }
  footer { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; padding: 12px 16px; border-top: 1px solid var(--line); flex-shrink: 0; }
  footer.editing { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  footer button { display: flex; align-items: center; justify-content: center; height: 36px; padding: 0 5px; font-size: 12px; background: var(--surface-alt); border: 1px solid var(--line); border-radius: var(--control-radius); }
  footer .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .snoozing .return { grid-column: 1 / -1; height: 28px; border: 0; background: transparent; color: var(--muted); }
  .field { display: block; font-size: 11px; color: var(--muted); margin-bottom: 14px; }
  .field:last-child { margin-bottom: 0; }
  .save-error { margin: 0; padding: 8px 16px; font-size: 12px; color: var(--danger); }
  input, textarea { width: 100%; margin-top: 6px; font: inherit; font-size: 13px; padding: 9px; border: 1px solid var(--line); border-radius: var(--control-radius); background: var(--field); color: var(--ink); }
  textarea { resize: none; min-height: 120px; line-height: 1.6; }
  .overflow-list button { display: flex; gap: 12px; align-items: center; width: 100%; justify-content: space-between; text-align: left; padding: 12px 0; border: 0; border-bottom: 1px solid var(--line); background: none; font-size: 12px; }
  .overflow-list button:last-child { border-bottom: 0; }
  .overflow-list span { min-width: 0; overflow-wrap: anywhere; }
footer button:hover:not(:disabled) { background: var(--accent-soft); } footer .primary:hover:not(:disabled) { background:var(--accent-hover); }
  .close { width:28px; height:28px; display:grid; place-items:center; border-radius:6px; }.close:hover { background:var(--surface-alt); }
</style>
