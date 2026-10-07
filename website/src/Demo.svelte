<script lang="ts">
  import { tick } from 'svelte';
  import EdgeStack from '../../frontend/src/components/EdgeStack.svelte';
  import QuickCard from '../../frontend/src/components/QuickCard.svelte';
  import type { Todo } from '../../frontend/src/lib/model';
  import { tomorrowMorning } from '../../frontend/src/lib/model';

  let { theme = 'mac' }: { theme?: 'mac' | 'paper' | 'graphite' } = $props();
  const makeTodos = (): Todo[] => [
    { id: 1, title: '把今天的灵感记下来', description: '一闪而过的好想法，也值得留一点位置。\n把它记下来，等有空时慢慢展开。', priority: 2, dueAt: 0, completed: false, completedAt: 0, snoozedUntil: 0 },
    { id: 2, title: '读完手边的那本书', description: '留出二十分钟，翻开上次停下的那一页。', priority: 0, dueAt: 0, completed: false, completedAt: 0, snoozedUntil: 0 },
    { id: 3, title: '周末去走走', description: '选一条没走过的路，暂时离开屏幕。', priority: 0, dueAt: 0, completed: false, completedAt: 0, snoozedUntil: 0 },
  ];
  let todos = $state(makeTodos());
  let phase = $state<'glance' | 'hover' | 'act'>('hover');
  let selected = $state(1);
  let current = $state(0);
  let editing = $state(false);
  let desktop = $state<HTMLDivElement>();
  let actButton = $state<HTMLButtonElement>();
  let opener: HTMLElement | null = null;
  let message = $state('试试右侧的任务标记，也可以使用下方按钮。');
  const activeTodos = $derived(todos.filter(todo => !todo.completed && !todo.snoozedUntil));
  const todo = $derived(todos.find(todo => todo.id === current));
  async function openCard(item: Todo, edit = false) {
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    current = item.id; editing = edit; phase = 'act';
    message = '在浮动卡片里直接处理这件事。';
    await tick();
    if (current === item.id && !editing) desktop?.querySelector<HTMLElement>('.quick-card')?.focus({ preventScroll: true });
  }
  function choosePhase(next: typeof phase) {
    phase = next; editing = false;
    if (next === 'act' && activeTodos[0]) void openCard(activeTodos[0]);
    else current = 0;
    selected = activeTodos[0]?.id ?? 0;
    message = next === 'glance' ? '平时只留一枚细窄标记，空白处没有背景层。' : next === 'hover' ? '悬停看清任务，再点击打开快速卡片。' : activeTodos.length ? '可以在卡片里完成、稍后或编辑。' : '演示任务已处理完，可以重新开始。';
  }
  function close() {
    current = 0; editing = false; phase = 'hover'; selected = activeTodos[0]?.id ?? 0;
    void tick().then(() => (opener?.isConnected ? opener : actButton)?.focus({ preventScroll: true }));
  }
  function complete(id: number) {
    todos = todos.map(todo => todo.id === id ? { ...todo, completed: true, completedAt: Date.now() } : todo);
    close(); message = '这件事完成了。演示操作只在当前页面生效。';
  }
  function reset() { todos = makeTodos(); current = 0; selected = 1; editing = false; phase = 'hover'; message = '演示已重置，所有示例任务都回来了。'; }
  function save(title: string, description: string) {
    todos = todos.map(todo => todo.id === current ? { ...todo, title: title.trim(), description } : todo);
    editing = false; message = '已更新演示任务，桌面应用数据没有改变。';
  }
  function snooze(duration: string) {
    const until = duration === 'tomorrow' ? tomorrowMorning(Date.now()) : Date.now() + (duration === '1h' ? 3600000 : 1800000);
    todos = todos.map(todo => todo.id === current ? { ...todo, snoozedUntil: until } : todo);
    close(); message = '这条演示任务暂时收起了，可以重置后再试。';
  }
</script>

<svelte:window onkeydown={event => { if (event.key === 'Escape' && current && !event.isComposing) { event.preventDefault(); if (editing) editing = false; else close(); } }} />
<div class="demo">
  <div class="desktop" data-theme={theme} bind:this={desktop}>
    <img class="wallpaper" src="./images/desktop-lake.webp" alt="雾气中的灰绿色山湖，作为互动侧栏演示的桌面背景" width="1440" height="901" fetchpriority="high" />
    <div class="desktop-thought" aria-hidden="true">给正在做的事，<br>多一点空间。</div>
    <div class="stack-viewport">
      <EdgeStack todos={activeTodos} side="right" offset={.47} height={380} width={312} itemHeight={44} mode="Passive" {selected} locked={phase === 'hover' ? selected : 0}
        onSelect={id => selected = id} onOpen={(item, _rect, edit) => void openCard(item, edit)}
        onComplete={complete} onOverflow={() => {}} onRegions={() => {}} />
    </div>
    {#if current && todo}
      <div class="card-position">
        <QuickCard {todo} {editing} onClose={close} onEdit={() => editing = true} onCancel={() => editing = false} onSave={save} onComplete={() => complete(current)} onSnooze={snooze} onChoose={item => current = item.id} />
      </div>
    {/if}
    {#if activeTodos.length === 0}<div class="empty-demo"><p>给今天留一点空白。</p><button onclick={reset}>重新开始演示</button></div>{/if}
  </div>
  <div class="demo-bar">
    <div class="demo-phases" aria-label="体验侧栏的三个状态">
      <button class:chosen={phase === 'glance'} aria-pressed={phase === 'glance'} onclick={() => choosePhase('glance')}>轻轻停靠</button>
      <button class:chosen={phase === 'hover'} aria-pressed={phase === 'hover'} onclick={() => choosePhase('hover')}>悬停查看</button>
      <button bind:this={actButton} class:chosen={phase === 'act'} aria-pressed={phase === 'act'} onclick={() => choosePhase('act')}>点击处理</button>
    </div>
    <button class="demo-reset" onclick={reset}>重置</button>
  </div>
  <p class="demo-hint" role="status">{message}</p>
</div>

<style>
  .desktop { position:relative; height:380px; overflow:hidden; border:1px solid var(--site-line); border-radius:16px; isolation:isolate; box-shadow:0 14px 40px #25342d0d; }
  .wallpaper { position:absolute; inset:0; width:100%; height:100%; object-fit:cover; }
  .desktop-thought { position:absolute; top:52px; left:34px; color:#f6f7f1; font-size:22px; line-height:1.6; font-weight:500; letter-spacing:.08em; text-shadow:0 1px 8px #18271e80; }
  .stack-viewport { position:absolute; inset:0 0 0 auto; width:312px; height:380px; pointer-events:none; }
  .stack-viewport :global([data-hit]) { pointer-events:auto; }
  .card-position { position:absolute; right:28px; top:68px; width:min(310px,calc(100% - 40px)); max-height:300px; }
  .card-position :global(.quick-card) { max-height:300px; box-shadow:0 8px 32px #1c30291f; }
  .card-position :global(.quick-card.editing) { height:300px; }
  .card-position :global(button) { color:var(--ink); cursor:pointer; }
  .card-position :global(button.primary) { color:var(--on-accent); }
  .empty-demo { position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center; background:#f6f7f2de; color:#25342d; }
  .empty-demo button { background:#38644f; color:#fff; border:0; border-radius:7px; padding:12px 18px; cursor:pointer; }
  .demo-bar { display:flex; align-items:center; justify-content:space-between; gap:8px; margin-top:16px; }
  .demo-phases { display:flex; gap:4px; }
  .demo-bar button { background:transparent; color:var(--site-muted); border:0; border-radius:7px; min-height:44px; padding:0 13px; font-size:12px; cursor:pointer; white-space:nowrap; }
  .demo-bar button.chosen { color:var(--site-accent); background:var(--site-accent-soft); }
  .demo-bar button:hover { color:var(--site-ink); }
  .demo-reset { text-decoration:underline; text-underline-offset:3px; }
  .demo-hint { min-height:36px; color:var(--site-muted); font-size:12px; line-height:1.6; margin:6px 0 0; }
  @media (max-width:767px) { .desktop { height:340px; } .stack-viewport { top:-20px; } .desktop-thought { top:32px; left:22px; font-size:18px; } .card-position { top:28px;right:20px; } .demo-bar button { padding-inline:8px; font-size:11px; } }
  @media (prefers-reduced-motion:reduce) { :global(.demo *) { transition:none!important; } }
</style>
