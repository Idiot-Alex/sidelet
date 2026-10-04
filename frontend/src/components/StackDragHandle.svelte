<script lang="ts">
  import { onMount } from 'svelte';
  import { request, send } from '../lib/bridge';
  import type { Rect } from '../lib/geometry';

  let { anchor, geometryKey, disabled = false }: { anchor: () => Rect; geometryKey: string; disabled?: boolean } = $props();
  let revision = $state(0);
  let busy = $state(false);
  let status = $state('');
  let pointer = -1, startX = 0, startY = 0, dx = 0, dy = 0, frame = 0;
  let capture: HTMLButtonElement | undefined;
  let original: Rect;
  let previousGeometry = '';
  let sequence = 0;
  const active = $derived(revision !== 0);
  function release() {
    cancelAnimationFrame(frame); frame = 0;
    const element = capture, id = pointer;
    pointer = -1; capture = undefined;
    if (element?.hasPointerCapture(id)) element.releasePointerCapture(id);
  }
  function cancel(notify = true) {
    const id = revision; revision = 0; release();
    if (id && notify) send('stack-drag-cancel', { revision: id });
    if (id) status = '已取消移动';
  }
  $effect(() => {
    if (geometryKey !== previousGeometry || disabled) cancel();
    previousGeometry = geometryKey;
  });
  function begin() {
    if (revision) return;
    revision = Date.now() * 100 + (++sequence % 100);
    status = '预览位置 · 松手或 Enter 保存 · Esc 取消';
    send('stack-drag-start', { revision, anchor: original });
  }
  function preview() {
    frame = 0;
    if (revision) send('stack-drag-preview', { revision, x: dx, y: dy });
  }
  function down(event: PointerEvent) {
    if (busy || disabled || event.button !== 0 || !event.isPrimary) return;
    cancel();
    const button = event.currentTarget as HTMLButtonElement;
    button.focus({ preventScroll: true });
    event.preventDefault();
    original = anchor();
    pointer = event.pointerId; startX = event.screenX; startY = event.screenY; dx = dy = 0;
    capture = button;
    try { button.setPointerCapture(pointer); } catch { cancel(); }
  }
  function move(event: PointerEvent) {
    if (event.pointerId !== pointer) return;
    dx = event.screenX - startX; dy = event.screenY - startY;
    if (!revision && Math.hypot(dx, dy) < 5) return;
    begin();
    if (!frame) frame = requestAnimationFrame(preview);
  }
  async function commit() {
    if (!revision || busy) return;
    const id = revision; revision = 0; release(); busy = true;
    status = '正在保存位置…';
    try { await request('stack-drag-end', { revision: id, x: dx, y: dy }); status = '位置已保存'; }
    catch (error) { status = error instanceof Error ? error.message : '保存失败，位置未更改。'; }
    finally { busy = false; }
  }
  function up(event: PointerEvent) {
    if (event.pointerId !== pointer) return;
    move(event);
    if (revision) void commit(); else release();
  }
  function key(event: KeyboardEvent) {
    if (busy || disabled || event.isComposing) return;
    if (event.key === 'Enter' && revision) { event.preventDefault(); event.stopPropagation(); void commit(); return; }
    if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
    event.preventDefault(); event.stopPropagation();
    if (pointer !== -1) return;
    if (!revision) { original = anchor(); dx = dy = 0; begin(); }
    if (event.key === 'ArrowLeft') dx = -100000;
    if (event.key === 'ArrowRight') dx = 100000;
    if (event.key === 'ArrowUp') dy -= event.shiftKey ? 100 : 20;
    if (event.key === 'ArrowDown') dy += event.shiftKey ? 100 : 20;
    preview();
  }
  onMount(() => {
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && (revision || pointer !== -1) && !event.isComposing) {
        event.preventDefault(); event.stopImmediatePropagation(); cancel();
      }
    };
    const blur = () => cancel();
    document.addEventListener('keydown', escape, true);
    window.addEventListener('blur', blur);
    let disposed = false, unsubscribe = () => {};
    void import('@wailsio/runtime').then(({ Events }) => {
      if (!disposed) unsubscribe = Events.On('stack:drag-cancelled', event => { if (Number(event.data) === revision) cancel(false); });
    });
    return () => { disposed = true; unsubscribe(); cancel(); document.removeEventListener('keydown', escape, true); window.removeEventListener('blur', blur); };
  });
</script>

<button class:active disabled={disabled || busy} aria-label="移动整组任务" title="拖动整组；方向键预览，Enter 保存，Esc 取消" data-hit onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={() => cancel()} onlostpointercapture={() => { if (pointer !== -1) cancel(); }} onkeydown={key}>⠿ <span>{busy ? '保存位置…' : active ? '松手保存 · Esc 取消' : '移动整组'}</span><span class="arrows" aria-hidden="true">↔ ↕</span></button>
<span class="sr-only" role="status">{status}</span>
{#if status.includes('失败')}<p class="error" role="alert">{status}</p>{/if}

<style>
  button { display: flex; align-items: center; gap: 9px; width: 100%; height: 36px; background: var(--surface-alt); color: var(--ink); border: 1px solid var(--line); border-radius: 5px; padding: 0 10px; font: inherit; font-size: 12px; cursor: grab; touch-action: none; user-select: none; }
  button.active { border-style: dashed; background: var(--accent-soft); cursor: grabbing; }
  button:focus-visible { outline: 2px solid var(--accent); outline-offset: -3px; }
  button:disabled { cursor: default; opacity: .6; }
  .arrows { margin-left: auto; color: var(--muted); }
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  .error { font-size: 11px; margin: 4px 0; padding: 6px; background: var(--danger-soft); color: var(--danger); }
</style>
