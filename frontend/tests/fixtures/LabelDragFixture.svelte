<script lang="ts">
  import { onDestroy } from 'svelte';
  import EdgeStack from '../../src/components/EdgeStack.svelte';
  import { initialSnapshot } from '../../src/lib/model';
  import type { Rect, Side } from '../../src/lib/geometry';
  let offset = $state(.35), saved = $state(.35), selected = $state(0);
  let side = $state<Side>('right'), savedSide = $state<Side>('right');
  let switches = $state(0), cancelOnPreview = $state(false);
  let originX = 0;
  let starts = $state(0), commits = $state(0), cancels = $state(0), opened = $state(0);
  let delta = $state({ x: 0, y: 0 });
  let revision = 0;
  let failSave = $state(false);
  let hits = $state<Rect[]>([]);
  const todos = initialSnapshot().todos.slice(0, 2).map(todo => ({ ...todo, dueAt: 0 }));
  type FixtureMessage = { type: string; revision?: number; requestId?: string; x?: number; y?: number; anchor?: Rect };
  type FixtureHost = { invoke: (data: string) => void; dispatchWailsEvent: (event: unknown) => void };
  const host = (window as unknown as { _wails: FixtureHost })._wails;
  const originalInvoke = host.invoke;
  function preview(msg: FixtureMessage) {
    delta = { x: msg.x!, y: msg.y! };
    offset = saved + delta.y / 1000;
    const nextSide = originX + delta.x >= innerWidth / 2 ? 'right' : 'left';
    if (side !== nextSide) switches++;
    side = nextSide;
  }
  function rollback() { offset = saved; side = savedSide; revision = 0; cancels++; }
  host.invoke = data => {
    const msg: FixtureMessage = JSON.parse(data);
    let error = '';
    if (msg.type === 'stack-drag-start') {
      revision = msg.revision!; starts++;
      originX = (savedSide === 'right' ? innerWidth - 312 : 0) + msg.anchor!.x + msg.anchor!.width / 2;
    }
    if (msg.type === 'stack-drag-preview' && revision === msg.revision) {
      preview(msg);
      if (cancelOnPreview) {
        const cancelled = revision;
        rollback();
        queueMicrotask(() => host.dispatchWailsEvent({ name: 'stack:drag-cancelled', data: cancelled }));
      }
    }
    if (msg.type === 'stack-drag-end') {
      if (!revision || revision !== msg.revision) error = '移动已取消';
      else if (failSave) { error = '测试保存失败'; offset = saved; side = savedSide; }
      else { preview(msg); saved = offset; savedSide = side; commits++; }
      revision = 0;
    }
    if (msg.type === 'stack-drag-cancel' && revision === msg.revision) rollback();
    if (msg.requestId) queueMicrotask(() => host.dispatchWailsEvent({ name: 'todo:result', sender: 'stack-0', data: { requestId: msg.requestId, error } }));
  };
  onDestroy(() => { host.invoke = originalInvoke; });
</script>

<div class="controls">
  <h1>标签拖动测试</h1><p>同一套 EdgeStack / StackDragHandle，测试宿主只保存内存状态。</p>
  <p role="status">开始 {starts} · 保存 {commits} · 取消 {cancels} · 打开卡片 {opened} · 位移 {delta.x}, {delta.y} · 已保存位置 {savedSide} {saved.toFixed(3)} · 预览 {side} · 换边 {switches}</p>
  <label><input type="checkbox" bind:checked={failSave}>模拟保存失败</label>
  <label><input type="checkbox" bind:checked={cancelOnPreview}>预览时模拟宿主取消</label>
  <p>命中区域 {hits.length} · 其余区域保持透明</p>
</div>
<div class="stage" class:left={side === 'left'}>
  <EdgeStack {todos} {side} {offset} height={1000} width={312} itemHeight={44} mode="Passive" {selected} locked={0} movable onSelect={id => selected = id} onOpen={() => opened++} onComplete={() => {}} onOverflow={() => {}} onRegions={rects => hits = rects} />
</div>

<style>
  :global(*) { box-sizing: border-box; }
  :global(body) { margin: 0; font-family: -apple-system, BlinkMacSystemFont, sans-serif; color: var(--ink); background: #e9ece7; }
  .controls { position: fixed; inset: 30px auto auto 30px; max-width: calc(100vw - 350px); }
  h1 { font-size: 20px; } p, label { font-size: 13px; }
  label { display: block; margin-block: 10px; }
  .stage { position: absolute; right: 0; top: 0; width: 312px; height: 1000px; pointer-events: none; }
  .stage.left { left: 0; right: auto; }
  .stage :global([data-hit]) { pointer-events: auto; }
</style>
