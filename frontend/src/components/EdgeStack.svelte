<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import TaskOrder from './TaskOrder.svelte';
  import StackDragHandle from './StackDragHandle.svelte';
  import type { DropTarget } from '../lib/reorder';
  import { arrangeStackLayout, stackItems, stackLayout, type Rect, type Side } from '../lib/geometry';
  import { dueLabel, dueState, type InputMode, type Todo } from '../lib/model';
  import type { NativePointer } from '../lib/bridge';

  let { todos, now = Date.now(), side, offset, height, width, viewportTop = 0, layoutRevision = 0, itemHeight = 44, mode, selected, locked, nativePointer, onSelect, onOpen, onComplete, onOverflow, onRegions, onPresence, onMetric, arranging = false, movable = false, orderError = "", onMove, onFinish, onShowAll }:
    { todos: Todo[]; now?: number; side: Side; offset: number; height: number; width: number; viewportTop?: number; layoutRevision?: number; itemHeight?: number; mode: InputMode; selected: number; locked: number; nativePointer?: NativePointer; onSelect: (id: number) => void; onOpen: (todo: Todo, rect: Rect, editing: boolean) => void; onComplete: (id: number) => void; onOverflow: (rect: Rect) => void; onRegions: (rects: Rect[], revision: number) => void; onPresence?: (inside: boolean) => void; onMetric?: (metric: { delayMs: number; fps: number; frameCount: number }) => void; arranging?: boolean; movable?: boolean; orderError?: string; onMove?: (id: number, target: DropTarget) => Promise<boolean>; onFinish?: () => void; onShowAll?: () => void } = $props();
  let root: HTMLDivElement;
  let expanded = $state(0);
  let moving = $state(false);
  let hover: ReturnType<typeof setTimeout> | undefined;
  let leave: ReturnType<typeof setTimeout> | undefined;
  let frame = 0;
  const layout = $derived(arranging ? arrangeStackLayout(todos.length, height, offset, itemHeight) : stackLayout(todos.length, height, offset, itemHeight));
  const active = $derived(moving ? 0 : mode === 'KeyboardActive' ? selected : locked || expanded);
  const items = $derived(stackItems(todos, layout.direct, arranging || moving ? 0 : mode === 'KeyboardActive' ? selected : locked));
  const dragGeometry = $derived(`${height}:${width}:${itemHeight}:${arranging}:${todos.map(todo => todo.id).join(',')}`);
  function movingChanged(value: boolean) {
    moving = value;
    if (value) { clearTimeout(hover); clearTimeout(leave); expanded = 0; }
  }
  let previousMode: InputMode = 'Passive';
  let previousLocked = 0;

  $effect(() => {
    if ((previousMode !== 'Passive' && mode === 'Passive') || (previousLocked && !locked)) {
      clearTimeout(hover); clearTimeout(leave); expanded = 0;
    }
    previousMode = mode; previousLocked = locked;
  });

  function enter(id: number) {
    if (arranging || moving) return;
    clearTimeout(leave); clearTimeout(hover);
    if (expanded === id) return;
    const started = performance.now();
    hover = setTimeout(() => {
      expanded = id; onSelect(id); cancelAnimationFrame(frame);
      let first = 0; let count = 0;
      void tick().then(() => {
        function sample(at: number) {
          if (!first) first = at;
          count++;
          if (at - first < 200) frame = requestAnimationFrame(sample);
          else onMetric?.({ delayMs: first - started, fps: (count - 1) * 1000 / (at - first), frameCount: count });
        }
        frame = requestAnimationFrame(sample);
      });
    }, 150);
  }
  function exit() {
    clearTimeout(hover); clearTimeout(leave);
    leave = setTimeout(() => { if (!locked && mode === 'Passive') expanded = 0; }, 500);
  }
  function localRect(element: Element): Rect {
    const bounds = element.getBoundingClientRect();
    const parent = root.parentElement!.getBoundingClientRect();
    return { x: bounds.x - parent.x, y: bounds.y - parent.y, width: bounds.width, height: bounds.height };
  }
  async function open(event: MouseEvent, todo: Todo, editing = false) {
    const element = (event.currentTarget as Element).closest('[data-hit]')!;
    expanded = todo.id;
    await tick();
    if (!element.isConnected) return;
    onOpen(todo, localRect(element), editing);
  }
  // macOS notices entry while the transparent panel still ignores mouse events.
  // This starts Hover even if the pointer stops before WebKit gets another move.
  $effect(() => {
    const pointer = nativePointer;
    if (arranging || moving || !pointer || !root) return;
    if (!pointer.inside) { exit(); return; }
    for (const element of root.querySelectorAll<HTMLElement>('[data-todo]')) {
      const rect = localRect(element);
      if (pointer.x >= rect.x && pointer.x < rect.x + rect.width && pointer.y >= rect.y && pointer.y < rect.y + rect.height) {
        enter(Number(element.dataset.todo)); break;
      }
    }
  });
  $effect(() => {
    active; moving; todos; width; height; side; offset; itemHeight; arranging; orderError; viewportTop;
    const revision = layoutRevision;
    let disposed = false;
    void tick().then(() => {
      if (!disposed && root) onRegions(Array.from(root.parentElement!.querySelectorAll('[data-hit]')).map(localRect), revision);
    });
    return () => { disposed = true; };
  });
  onDestroy(() => { clearTimeout(hover); clearTimeout(leave); cancelAnimationFrame(frame); });
</script>

{#if arranging}
  <div class="stack-drag-handle" style:top={`${layout.top - viewportTop - 42}px`} style:left={side === 'left' ? '0' : 'auto'} style:right={side === 'right' ? '0' : 'auto'} style:width={`${Math.max(0, Math.min(296, width - 20))}px`} data-sidelet>
    <StackDragHandle anchor={() => localRect(root)} geometryKey={dragGeometry} disabled={height < 190} />
  </div>
{/if}
<div bind:this={root} class="edge-stack" class:left={side === 'left'} style:top={`${layout.top - viewportTop}px`} style:--item-height={`${layout.rowHeight}px`} style:width={arranging ? `${Math.max(0, Math.min(296, width - 20))}px` : undefined} data-sidelet>
  {#if arranging && onMove}
    <div style:width={`${Math.max(0, Math.min(296, width - 20))}px`}><TaskOrder todos={items.direct} rowHeight={layout.rowHeight} {onMove} /></div>
  {:else}
  {#each items.direct as todo (todo.id)}
    <div class="edge-row" role="group" aria-label={todo.title} class:expanded={active === todo.id} class:completed={todo.completed} class:soon={dueState(todo, now) === 'soon'} class:overdue={dueState(todo, now) === 'overdue'} style:width={`${active === todo.id ? Math.max(0, Math.min(296, width - 20)) : 14}px`} data-hit data-todo={todo.id} onpointerenter={() => { onPresence?.(true); enter(todo.id); }} onpointerleave={() => { onPresence?.(false); exit(); }}>
      {#if active === todo.id}
        <button class="check" aria-label={`完成${todo.title}`} onpointerenter={() => enter(todo.id)} onpointerleave={exit} onclick={() => onComplete(todo.id)} disabled={todo.completed}>{todo.completed ? '✓' : ''}</button>
      {/if}
      <button class="edge-label" aria-label={`查看${todo.title}${dueLabel(todo, now) ? ' · ' + dueLabel(todo, now) : ''}`} onpointerenter={() => enter(todo.id)} onpointerleave={exit} onclick={event => open(event, todo)} ondblclick={event => open(event, todo, true)}>
        {#if active === todo.id}
          <span class="edge-title">{todo.completed ? '已完成' : todo.title}</span>
          {#if todo.dueAt}<time>{dueLabel(todo, now)} {new Date(todo.dueAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}</time>{/if}
        {/if}
      </button>
      <StackDragHandle compact label={`移动整组任务：${todo.title}`} anchor={() => localRect(root)} geometryKey={dragGeometry} disabled={!movable || mode === 'Editing'} onEngaged={movingChanged} />
    </div>
  {/each}
  {/if}
  {#if layout.overflow > 0}
    <div class="overflow-row" data-hit>
      <button class="overflow-tab" aria-label={`查看其余${layout.overflow}项任务`} onpointerenter={() => { clearTimeout(leave); onPresence?.(true); }} onpointerleave={() => { exit(); onPresence?.(false); }} onclick={event => arranging ? onShowAll?.() : onOverflow(localRect(event.currentTarget))}>+{layout.overflow}</button>
      {#if !arranging}<StackDragHandle compact label="移动整组任务：更多任务" anchor={() => localRect(root)} geometryKey={dragGeometry} disabled={!movable} onEngaged={movingChanged} />{/if}
    </div>
  {/if}
</div>
{#if arranging}
  <div class="arrange-toolbar" style:left={side === "left" ? "0" : "auto"} style:right={side === "right" ? "0" : "auto"} style:width={`${Math.max(0, Math.min(296, width - 20))}px`} data-hit data-sidelet>
    <div><strong>整理桌面</strong><button onclick={onShowAll}>全部任务</button><button onclick={onFinish}>完成整理</button></div>
    <p class:error={!!orderError} role="status">{orderError || "上方移动整组 · 行内手柄排序 · Esc 退出"}</p>
  </div>
{/if}

<style>
  .stack-drag-handle { position: absolute; z-index: 2; }
  .arrange-toolbar { position: absolute; bottom: 10px; height: 70px; padding: 9px 10px; background: var(--surface); border: 1px solid var(--line); border-radius: 5px; }
  .arrange-toolbar div { display: flex; align-items: center; gap: 8px; }.arrange-toolbar strong { flex: 1; font-size: 12px; font-weight: 500; }
  .arrange-toolbar button { font-size: 11px; color: var(--accent); background: var(--surface-alt); border: 0; border-radius: 3px; padding: 5px; }.arrange-toolbar p { font-size: 10px; margin: 7px 0 0; line-height: 1.4; color: var(--muted); }.arrange-toolbar .error { color: var(--danger); }
  .edge-stack { position: absolute; right: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 6px; }
  .edge-stack.left { right: auto; left: 0; align-items: flex-start; }
  .edge-row { position: relative; height: var(--item-height); display: flex; align-items: center; border: 1px solid var(--line); background: var(--surface-alt); border-radius: 5px 0 0 5px; }
  .left .edge-row { border-radius: 0 5px 5px 0; }
  .edge-row.expanded { background: var(--surface); border-color: var(--line); }
  .edge-label { border: 0; background: none; padding: 0; display: flex; align-items: center; gap: 10px; min-width: 0; flex: 1; height: 100%; text-align: left; }
  .edge-row:not(.expanded) .edge-label { display: none; }
  .edge-label { padding-inline: 4px; }
  .left .edge-row :global(button.compact), .left .overflow-row :global(button.compact) { order: -1; }
  .edge-row :global(button.compact) { flex-basis: 12px; width: 12px; border-radius: 0; }
  .edge-row.expanded :global(button.compact) { flex-basis: 20px; width: 20px; }
  .soon :global(button.compact) { color: var(--warning); }
  .overdue :global(button.compact) { color: var(--danger); }
  .edge-title { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .expanded .edge-title { animation: reveal 200ms ease-out; }
  @keyframes reveal { from { opacity: 0; transform: translateX(8px); } to { opacity: 1; transform: translateX(0); } }
  @media (prefers-reduced-motion: reduce) { .expanded .edge-title { animation: none; } }
  time { font-size: 10px; color: var(--muted); font-variant-numeric: tabular-nums; }
  .soon time { color: var(--warning); } .overdue time { color: var(--danger); }
  .check { width: 16px; height: 16px; border: 1px solid var(--line); border-radius: 4px; padding: 0; margin: 0 10px 0 12px; background: var(--surface); color: var(--accent); font-size: 11px; flex-shrink: 0; }
  .check:hover { background: var(--accent-soft); }
  .completed { opacity: 0.65; }
  .overflow-row { position: relative; display: flex; height: var(--item-height); border: 1px solid var(--line); border-radius: 5px 0 0 5px; background: var(--surface-alt); }
  .overflow-tab { height: 100%; min-width: 36px; border: 0; border-radius: inherit; color: var(--muted); background: none; padding: 0 7px; font-size: 10px; }
  .left .overflow-row { border-radius: 0 5px 5px 0; }
</style>
