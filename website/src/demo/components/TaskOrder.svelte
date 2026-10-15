<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Todo } from '../lib/model';
  import { dropTarget, type DropTarget } from '../lib/reorder';

  let { todos, rowHeight = 44, onMove }:
    { todos: Todo[]; rowHeight?: number; onMove: (id: number, target: DropTarget) => Promise<boolean> } = $props();
  let root: HTMLUListElement;
  let source = $state(0);
  let dragging = $state(false);
  let target = $state<DropTarget>();
  let busy = $state(false);
  let status = $state('');
  let capture: HTMLElement | undefined;
  let pointerID = -1;
  let startX = 0, startY = 0;
  let lastX = 0, lastY = 0;
  let scrollFrame = 0;
  let signature = '';
  let measuredHeight = 0;
  const currentSignature = $derived(todos.map(todo => `${todo.id}:${todo.sortOrder}:${todo.completed}:${todo.snoozedUntil}`).join('|'));

  function cancel() {
    cancelAnimationFrame(scrollFrame); scrollFrame = 0;
    const element = capture, id = pointerID;
    capture = undefined; pointerID = -1; source = 0; dragging = false; target = undefined;
    if (element?.hasPointerCapture(id)) element.releasePointerCapture(id);
  }
  $effect(() => {
    if (signature !== currentSignature || measuredHeight !== rowHeight) cancel();
    signature = currentSignature;
    measuredHeight = rowHeight;
  });
  function down(event: PointerEvent, id: number) {
    if (busy || todos.length < 2 || event.button !== 0 || !event.isPrimary) return;
    event.preventDefault();
    cancel(); status = ''; source = id; startX = event.clientX; startY = event.clientY;
    capture = event.currentTarget as HTMLElement; pointerID = event.pointerId;
    try { capture.setPointerCapture(pointerID); } catch { cancel(); }
  }
  function move(event: PointerEvent) {
    if (event.pointerId !== pointerID || !source) return;
    if (!dragging && Math.hypot(event.clientX - startX, event.clientY - startY) < 5) return;
    dragging = true;
    lastX = event.clientX; lastY = event.clientY;
    updateTarget();
    if (!scrollFrame) scrollFrame = requestAnimationFrame(autoScroll);
  }
  function updateTarget() {
    const box = root.getBoundingClientRect();
    if (lastX < box.left || lastX > box.right || lastY < box.top - 12 || lastY > box.bottom + 12) { target = undefined; return; }
    target = dropTarget(Array.from(root.querySelectorAll<HTMLElement>('[data-order-id]')).map(row => {
      const bounds = row.getBoundingClientRect();
      return { id: Number(row.dataset.orderId), top: bounds.top, bottom: bounds.bottom };
    }), source, lastY);
  }
  function autoScroll() {
    scrollFrame = 0;
    if (!source || !dragging) return;
    let scroller: Element = document.scrollingElement ?? document.documentElement;
    for (let parent = root.parentElement; parent; parent = parent.parentElement) {
      if (parent.scrollHeight > parent.clientHeight && /auto|scroll/.test(getComputedStyle(parent).overflowY)) { scroller = parent; break; }
    }
    const box = root.getBoundingClientRect();
    const viewport = scroller === document.scrollingElement ? { top: 0, bottom: innerHeight } : scroller.getBoundingClientRect();
    if (lastX >= box.left && lastX <= box.right) {
      const speed = lastY < viewport.top + 44 ? -Math.min(18, (viewport.top + 44 - lastY) / 2) : lastY > viewport.bottom - 44 ? Math.min(18, (lastY - viewport.bottom + 44) / 2) : 0;
      if (speed) { scroller.scrollTop += speed; updateTarget(); }
    }
    scrollFrame = requestAnimationFrame(autoScroll);
  }
  async function commit(id: number, destination: DropTarget, restoreFocus = false) {
    busy = true; status = '正在保存顺序…';
    try { status = await onMove(id, destination) ? '顺序已保存' : '保存失败，顺序未更改。请重试。'; }
    finally {
      busy = false;
      if (restoreFocus) {
        await tick();
        if (document.hasFocus()) root?.querySelector<HTMLButtonElement>(`[data-order-id="${id}"] .handle`)?.focus({ preventScroll: true });
      }
    }
  }
  function up(event: PointerEvent) {
    if (event.pointerId !== pointerID) return;
    move(event);
    const id = source, destination = dragging ? target : undefined;
    cancel();
    if (destination) void commit(id, destination);
  }
  function key(event: KeyboardEvent, index: number) {
    if (!event.altKey || !['ArrowUp', 'ArrowDown'].includes(event.key)) return;
    event.preventDefault(); event.stopPropagation();
    if (busy || source) return;
    const other = todos[index + (event.key === 'ArrowUp' ? -1 : 1)];
    if (other) void commit(todos[index].id, { id: other.id, after: event.key === 'ArrowDown' }, true);
  }
  onMount(() => {
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !source) return;
      event.preventDefault(); event.stopImmediatePropagation(); cancel(); status = '已取消拖动';
    };
    document.addEventListener('keydown', escape, true);
    window.addEventListener('blur', cancel);
    window.addEventListener('resize', cancel);
    return () => { cancel(); document.removeEventListener('keydown', escape, true); window.removeEventListener('blur', cancel); window.removeEventListener('resize', cancel); };
  });
</script>

<ul bind:this={root} aria-label="桌面任务顺序" aria-busy={busy} style:--row-height={`${rowHeight}px`}>
  {#each todos as todo, index (todo.id)}
    <li data-hit data-order-id={todo.id} class:dragging={dragging && source === todo.id} class:before={target?.id === todo.id && !target.after} class:after={target?.id === todo.id && target.after}>
      <button class="handle" aria-label={`拖动排序${todo.title}`} title="拖动排序；Option + ↑ / ↓ 也可调整" disabled={busy || todos.length < 2} onpointerdown={event => down(event, todo.id)} onpointermove={move} onpointerup={up} onpointercancel={cancel} onlostpointercapture={cancel} onkeydown={event => key(event, index)}>⠿</button>
      <span class="title">{todo.title}</span><span class="position" aria-hidden="true">{index + 1}</span>
    </li>
  {/each}
</ul>
<span class="sr-only" role="status">{status}</span>

<style>
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; width: 100%; user-select: none; }
  li { position: relative; height: var(--row-height); display: flex; align-items: center; gap: 8px; background: var(--surface); border: 1px solid var(--line); border-radius: 5px; }
  li.dragging { background: var(--surface-alt); border-style: dashed; }
  li.before::before, li.after::after { content: ''; position: absolute; left: 0; right: 0; height: 3px; background: var(--accent); border-radius: 2px; pointer-events: none; }
  li.before::before { top: -5px; } li.after::after { bottom: -5px; }
  .handle { width: 32px; height: 100%; flex-shrink: 0; border: 0; background: transparent; color: var(--muted); font-size: 22px; cursor: grab; touch-action: none; }
  .handle:active { cursor: grabbing; }.handle:disabled { cursor: default; opacity: .4; }
  .handle:focus-visible { outline: 2px solid var(--accent); outline-offset: -3px; border-radius: 4px; }
  .title { flex: 1; min-width: 0; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; font-size: 12px; color: var(--ink); }
  .position { padding-right: 12px; font-size: 10px; color: var(--subtle); }
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
</style>
