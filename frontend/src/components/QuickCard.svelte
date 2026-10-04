<script lang="ts">
  import Icon from './Icon.svelte';
  import { onMount, tick } from 'svelte';
  import { trackMemory } from '../lib/memory';
  onMount(() => trackMemory('QuickCard'));
  import { dueLabel, type Todo } from '../lib/model';
  let { todo, overflow = [], editing, busy = false, error = '', now = Date.now(), onClose, onEdit, onSave, onComplete, onSnooze, onChoose, onCancel, onPresence }:
    { todo?: Todo; overflow?: Todo[]; editing: boolean; busy?: boolean; error?: string; now?: number; onClose: () => void; onEdit: () => void; onSave: (title: string, description: string) => void; onComplete: () => void; onSnooze: (duration: string) => void; onChoose: (todo: Todo) => void; onCancel: () => void; onPresence?: (inside: boolean) => void } = $props();
  let title = $state('');
  let description = $state('');
  let snooze = $state(false);
  let input = $state<HTMLInputElement>();
  let loadedID = 0;
  let wasEditing = false;
  $effect(() => {
    if (editing && todo && (!wasEditing || loadedID !== todo.id)) {
      loadedID = todo.id;
      title = todo.title; description = todo.description;
      void tick().then(() => { input?.focus(); input?.select(); });
    }
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

<div class="quick-card" role="dialog" tabindex="-1" aria-label="任务快速操作" data-sidelet data-hit onkeydown={key} onpointerenter={() => onPresence?.(true)} onpointerleave={() => onPresence?.(false)}>
  <header><span class="eyebrow">{overflow.length ? '更多任务' : editing ? '快速编辑' : '当前任务'}</span><button class="close" aria-label="关闭快速卡片" onclick={onClose} disabled={busy}><Icon name="close" size={16} /></button></header>
  {#if overflow.length}
    <div class="quick-body overflow-list">
      {#each overflow as item}<button onclick={() => onChoose(item)}>{item.title}<span>↗</span></button>{/each}
    </div>
  {:else if todo}
    <div class="quick-body">
      {#if editing}
        <label class="field">任务标题<input bind:this={input} bind:value={title} maxlength="500" disabled={busy} /></label>
        <label class="field">备注<textarea bind:value={description} rows="6" disabled={busy}></textarea></label>
      {:else}
        <h2>{todo.title}</h2>
        {#if todo.dueAt}<p class="due">{dueLabel(todo, now) ? dueLabel(todo, now) + ' · ' : ''}{new Date(todo.dueAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })}</p>{/if}
        <p class="description">{todo.description}</p>
      {/if}
    </div>
    {#if error}<p class="save-error" role="alert">{error}</p>{/if}
    <footer>
      {#if editing}
        <button class="primary" onclick={() => onSave(title, description)} disabled={busy || !title.trim()}>{busy ? '保存中…' : '保存'}</button><button onclick={onCancel} disabled={busy}>取消</button>
      {:else if snooze}
        <div class="snooze-options"><button disabled={busy} onclick={() => onSnooze('30m')}>30 分钟</button><button disabled={busy} onclick={() => onSnooze('1h')}>1 小时</button><button disabled={busy} onclick={() => onSnooze('tomorrow')}>明天 09:00</button><button disabled={busy} aria-label="返回操作" onclick={() => snooze = false}>返回</button></div>
      {:else}
        <button class="primary" onclick={onComplete} disabled={busy}>✓ 完成</button><button onclick={() => snooze = true} disabled={busy}>稍后</button><button onclick={onEdit} disabled={busy}>编辑</button>
      {/if}
    </footer>
  {:else}
    <div class="quick-body"><p>选择一条任务开始操作。</p></div>
  {/if}
</div>

<style>
  .quick-card { height: 100%; display: flex; flex-direction: column; background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); overflow: hidden; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 15px 20px 6px; flex-shrink: 0; }
  .eyebrow { font: 11px var(--font-ui); letter-spacing: .03em; color: var(--muted); }
  .close { padding: 0; border: 0; background: transparent; font-size: 22px; color: var(--muted); }
  .quick-body { flex: 1; min-height: 0; overflow-y: auto; padding: 14px 20px; }
  h2 { position: sticky; top: 0; z-index: 1; background: var(--surface); font-family: var(--font-heading); font-size: 21px; line-height: 1.5; font-weight: var(--heading-weight); margin: 0 0 12px; overflow-wrap: anywhere; }
  .due { font-size: 11px; color: var(--muted); padding-bottom: 15px; border-bottom: 1px solid var(--line); }
  .description { white-space: pre-wrap; font-size: 13px; line-height: 1.8; color: var(--muted); overflow-wrap: anywhere; }
  footer { display: flex; gap: 7px; padding: 14px 16px; border-top: 1px solid var(--line); flex-shrink: 0; }
  footer button { padding: 7px 12px; font-size: 12px; background: var(--surface-alt); border: 1px solid var(--line); border-radius: var(--control-radius); }
  footer .primary { background: var(--accent); color: var(--on-accent); border-color: var(--accent); }
  .snooze-options { display: flex; flex-wrap: wrap; gap: 6px; }
  .field { display: block; font-size: 11px; color: var(--muted); margin-bottom: 14px; }
  .save-error { margin: 0; padding: 8px 20px; font-size: 12px; color: var(--danger); }
  input, textarea { width: 100%; margin-top: 6px; font: inherit; font-size: 13px; padding: 9px; border: 1px solid var(--line); border-radius: var(--control-radius); background: var(--field); color: var(--ink); }
  textarea { resize: none; }
  .overflow-list button { display: flex; width: 100%; justify-content: space-between; text-align: left; padding: 12px 0; border: 0; border-bottom: 1px solid var(--line); background: none; font-size: 12px; }
  .overflow-list span { color: var(--subtle); }
footer button:hover:not(:disabled) { background: var(--accent-soft); } footer .primary:hover:not(:disabled) { background:var(--accent-hover); }
  .close { width:28px; height:28px; display:grid; place-items:center; border-radius:6px; }.close:hover { background:var(--surface-alt); }
</style>
