<script lang="ts">
  import { onMount, tick, untrack } from 'svelte';
  import EdgeStack from './components/EdgeStack.svelte';
  import QuickCard from './components/QuickCard.svelte';
  import QuickAdd from './components/QuickAdd.svelte';
  import SettingsPanel from './components/SettingsPanel.svelte';
 import type { SettingsState } from './lib/settings';
 import TodoManager from './components/TodoManager.svelte';
  import { moveTask, type DropTarget } from './lib/reorder';
  import { trackMemory } from './lib/memory';
  import { ensureVisible, quickRect, stackItems, stackLayout, type Rect, type Side } from './lib/geometry';
  import { connect, dispatch, fixtureMode, hitRegions, hostPlatform, native, role, send, shortcutLabel, type QuickAddState, type NativePointer, type NotificationStatus } from './lib/bridge';
  import { initialSnapshot, nextDeadline, reduce, visibleTodos, type Action, type InputMode, type Todo } from './lib/model';

  let snapshot = $state(native && !fixtureMode ? { todos: [], stacks: [], storage: 'sqlite' as const, undoId: 0, undoUntil: 0, selectedId: 0 } as ReturnType<typeof initialSnapshot> : initialSnapshot());
  let ready = $state(!native);
  let pendingCount = $state(0);
  let now = $state(Date.now());
  let mode = $state<InputMode>('Passive');
  let nativePointer = $state<NativePointer>();
  let side = $state<Side>('right');
  let offset = $state(0.35);
  let height = $state(role === 'stack' || role === 'quick' ? innerHeight : 520);
  let width = $state(role === 'stack' || role === 'quick' ? innerWidth : 700);
  let itemHeight = $state(44);
  let workHeight = $state(0);
  let viewportTop = $state(0);
  let layoutRevision = $state(0);
  const stackHeight = $derived(role === 'stack' && hostPlatform === 'darwin' && workHeight > 0 ? workHeight : height);
  let stackIndex = $state(0);
  let stackCount = $state(1);
  let stackId = $state(0);
  let selected = $state(1);
  let quickOpen = $state(false);
  let quickSource = $state(-1);
  let quickTask = $state(0);
  let sourceInside = $state(false);
  let cardInside = $state(false);
  let overflowOpen = $state(false);
  let quickBounds = $state<Rect>({ x: 0, y: 0, width: 320, height: 390 });
  let regions = $state<Rect[]>([]);
  let showRegions = $state(false);
  let quiet = $state(false);
  let arranging = $state(false);
  let error = $state('');
  let addSession = $state<QuickAddState>({ open:false, saving:false, revision:0, resetVersion:0 });
  let settingsOpen = $state(false);
  let controlVisible = $state(true);
  let controlRevision = 0;
  let controlScroll = { x: 0, y: 0, settings: false };
  let controlFocus: HTMLElement | null = null;
  function controlVisibility(value: { visible: boolean; revision: number }) {
    if (hostPlatform !== 'darwin' || role !== 'control' || value.revision < controlRevision) return;
    controlRevision = value.revision;
    const restore = value.visible && !controlVisible;
    if (!value.visible && controlVisible) {
      controlScroll = { x: scrollX, y: scrollY, settings: settingsOpen };
      controlFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    }
    controlVisible = value.visible;
    void tick().then(() => {
      if (controlVisible !== value.visible || controlRevision !== value.revision) return;
      if (restore) {
        if (settingsOpen === controlScroll.settings) {
          if (controlFocus?.isConnected && !controlFocus.closest('[hidden]')) controlFocus.focus({ preventScroll: true });
          window.scrollTo(controlScroll.x, controlScroll.y);
        }
        controlFocus = null;
      }
      send(value.visible ? 'control-rendered' : 'control-hidden', { revision: value.revision });
    });
  }
 let settingsState = $state<SettingsState>();
  $effect(() => { document.documentElement.dataset.theme = settingsState?.value.appearance.theme ?? 'mac'; });
  $effect(() => { settingsOpen; void tick().then(() => window.scrollTo(0, 0)); });
 let notificationStatus = $state<NotificationStatus>({ authorization: '', message: '' });
  let underlyingClicks = $state(0);
  let eventLog = $state<string[]>([]);
  let previous: HTMLElement | null = null;
  let editReturnMode: InputMode = 'Passive';
  let canvas = $state<HTMLDivElement>();
  let keyboardRoot = $state<HTMLDivElement>();

  const allVisible = $derived(visibleTodos(snapshot, now).filter(todo => !arranging || !todo.completed));
  const todos = $derived(role === 'stack' || role === 'quick' ? allVisible.filter(todo => snapshot.storage === 'sqlite' ? todo.stackId === stackId : stackCount === 1 || (todo.id - 1) % stackCount === stackIndex) : allVisible);
  const actionable = $derived(todos.filter(todo => !todo.completed));
  const layout = $derived(stackLayout(todos.length, stackHeight, offset, itemHeight));
  const current = $derived(snapshot.todos.find(todo => todo.id === (role === 'quick' ? snapshot.selectedId : selected)));
  const area = $derived({ x: 0, y: 0, width, height });
  const locked = $derived(quickOpen && (!native || quickSource === stackIndex) ? quickTask : 0);
  const items = $derived(stackItems(todos, layout.direct, mode === 'KeyboardActive' ? selected : locked));
  const overflow = $derived(items.overflow);
  const cardOverflow = $derived(role === 'quick' ? snapshot.todos.filter(todo => snapshot.overflowIds?.includes(todo.id)) : overflow);

  function arrange(enabled: boolean) { if (native) send('arrange', { enabled }); }
  async function reorder(id: number, target: DropTarget): Promise<boolean> {
    const source = snapshot.todos.find(todo => todo.id === id);
    if (!arranging || !source?.stackId || pendingCount) return false;
    const previousOrder = visibleTodos(snapshot, Date.now()).filter(todo => !todo.completed && todo.stackId === source.stackId).map(todo => todo.id);
    const order = moveTask(previousOrder, id, target);
    if (order.every((value, index) => value === previousOrder[index])) return true;
    return act({ type: 'reorder', stackId: source.stackId, previousOrder, order });
  }
  function record(text: string) { eventLog = [text, ...eventLog].slice(0, 5); }
  function metric(value: { delayMs: number; fps: number; frameCount: number }) {
    if (native) send('metric', { metric: value });
    else record(`Hover ${Math.round(value.delayMs)}ms · 帧回调 ${Math.round(value.fps)}fps`);
  }
  async function act(action: Action): Promise<boolean> {
    error = ''; pendingCount++;
    try {
      if (native) await dispatch(action); else { snapshot = reduce(snapshot, action); now = Date.now(); }
      record(({ complete: '任务已完成，5 秒内可撤销', undo: '已撤销完成', snooze: '任务已暂时隐藏', edit: '任务已更新', reset: '已恢复 8 条假数据' } as Record<string, string>)[action.type] ?? action.type);
      return true;
    } catch (cause) { error = cause instanceof Error ? cause.message : '操作失败，请重试。'; return false; }
    finally { pendingCount--; }
  }
  async function inputMode(next: InputMode, restore = false) {
    if (quiet && next !== 'Passive') return;
    if (native) { send('mode', { mode: next }); return; }
    const wasActive = mode !== 'Passive';
    if (mode === 'Passive' && next !== 'Passive') previous = document.activeElement as HTMLElement;
    mode = next;
    if (next === 'KeyboardActive') { await tick(); (keyboardRoot ?? canvas)?.focus({ preventScroll: true }); }
    if (next === 'Passive' && restore && wasActive) previous?.focus({ preventScroll: true });
  }
  function choose(id: number) { if (!quickOpen && mode === 'Passive') selected = id; }
  function open(todo: Todo, rect: Rect, editing = false) {
    selected = todo.id; overflowOpen = false;
    if (native) {
      send('quick', { action: { type: 'select', id: todo.id }, anchor: rect, mode: editing ? 'Editing' : mode });
    } else {
      quickBounds = quickRect(rect, area, side); quickOpen = true; quickTask = todo.id; sourceInside = true; cardInside = false;
      if (editing) startEditing();
    }
    record(editing ? '打开快速编辑' : '打开 Quick Card');
  }
  function openOverflow(rect: Rect) {
    if (native) { send('overflow', { ids: overflow.map(todo => todo.id), anchor: rect, mode }); return; }
    quickBounds = quickRect(rect, area, side); overflowOpen = true; quickOpen = true; quickTask = 0; sourceInside = true; cardInside = false;
  }
  function closeQuick(restore = true) {
    if (native) send('close-quick');
    quickOpen = false; overflowOpen = false;
    quickTask = 0; sourceInside = false; cardInside = false;
    if (!native) void inputMode('Passive', restore);
  }
  function startEditing() { editReturnMode = mode === 'KeyboardActive' ? 'KeyboardActive' : 'Passive'; void inputMode('Editing'); }
  async function finishEditing(save: boolean, title = '', description = '') {
    if (pendingCount) return;
    if (save && current && !(await act({ type: 'edit', id: current.id, title, description }))) return;
    void inputMode(editReturnMode, editReturnMode === 'Passive');
  }
  async function complete(id: number) { if (await act({ type: 'complete', id })) { if (quickOpen || role === 'quick') closeQuick(); } }
  async function snooze(duration: string) { if (current && await act({ type: 'snooze', id: current.id, duration })) { if (quickOpen || role === 'quick') closeQuick(); } }
  function reset() { act({ type: 'reset' }); closeQuick(false); }
  function toggleQuiet() {
    if (native) { send('quiet'); return; }
    quiet = !quiet;
    closeQuick();
  }
  function updateSide(value: Side) { side = value; if (native) send('side', { side }); closeQuick(false); }
  function presence(surface: 'source' | 'card', inside: boolean) {
    if (native) { send('presence', { inside }); return; }
    if (surface === 'source') sourceInside = inside; else cardInside = inside;
  }
  function receiveRegions(rects: Rect[], revision = layoutRevision) {
    if (revision !== layoutRevision) return;
    regions = rects;
    if (role === 'stack') {
      const undo = document.querySelector('.undo-toast')?.getBoundingClientRect();
      hitRegions(undo ? [...rects, { x: undo.x, y: undo.y, width: undo.width, height: undo.height }] : rects, revision);
    }
  }
  function keyboard(event: KeyboardEvent) {
    if (role === 'add') return;
 if (role === "control" && snapshot.storage === "sqlite" && event.metaKey && event.key === ",") { event.preventDefault(); error = ""; settingsOpen = true; send("settings-refresh"); return; }
    if (arranging) {
      if (event.key === 'Escape' && !event.isComposing) { event.preventDefault(); arrange(false); }
      return;
    }
    if (!native && !quiet && event.ctrlKey && event.altKey && event.key.toLowerCase() === 't') { event.preventDefault(); void inputMode('KeyboardActive'); return; }
    if (mode !== 'KeyboardActive' || event.isComposing) return;
    const target = event.target;
    if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || (target instanceof HTMLElement && target.isContentEditable)) return;
    const key = event.key.toLowerCase();
    if (!['arrowup', 'arrowdown', 'enter', ' ', 's', 'e', 'escape'].includes(key)) return;
    event.preventDefault();
    if (key === 'escape') { closeQuick(); void inputMode('Passive', true); return; }
    if (key === 'arrowup' || key === 'arrowdown') {
      const index = Math.max(0, actionable.findIndex(todo => todo.id === selected));
      selected = actionable[(index + (key === 'arrowdown' ? 1 : actionable.length - 1)) % actionable.length]?.id ?? 0;
      if (quickOpen && !native) quickTask = selected;
      if (role === 'quick') act({ type: 'select', id: selected });
    }
    if (!current) return;
    if (key === ' ') complete(current.id);
    if (key === 's') snooze('30m');
    if (key === 'enter' || key === 'e') {
      if (role === 'quick') { if (key === 'e') startEditing(); return; }
      const index = Math.max(0, items.direct.findIndex(todo => todo.id === selected));
      let rect = { x: side === 'right' ? width - 14 : 0, y: layout.top - viewportTop + index * (itemHeight + 6), width: 14, height: itemHeight };
      const parent = role === 'stack' ? keyboardRoot : canvas;
      const row = parent?.querySelector(`[data-todo="${selected}"]`);
      if (row && parent) {
        const bounds = row.getBoundingClientRect();
        const origin = parent.getBoundingClientRect();
        rect = { x: bounds.x - origin.x, y: bounds.y - origin.y, width: bounds.width, height: bounds.height };
      }
      open(current, rect, key === 'e');
    }
  }
  function outside(event: PointerEvent) {
    if (!native && mode !== 'Passive' && !(event.target as Element).closest('[data-sidelet]')) {
      quickOpen = false; mode = 'Passive'; record('点击外部，已退出键盘模式');
    }
  }
  $effect(() => {
    if (!actionable.some(todo => todo.id === selected)) selected = actionable[0]?.id ?? 0;
  });
  $effect(() => {
    if (native || !quickOpen || mode !== 'Passive' || sourceInside || cardInside) return;
    const timer = setTimeout(() => closeQuick(false), 500);
    return () => clearTimeout(timer);
  });
  $effect(() => {
    const deadline = nextDeadline(snapshot, now);
    if (deadline === undefined) return;
    const timer = setTimeout(() => { now = Date.now(); }, Math.min(2147483647, Math.max(1, deadline - Date.now())));
    return () => clearTimeout(timer);
  });
  $effect(() => {
    height; width; quickOpen;
    if (!native && quickOpen) quickBounds = ensureVisible(untrack(() => quickBounds), area);
  });
  $effect(() => {
    snapshot.undoUntil; now;
    if (role === 'stack') void tick().then(() => receiveRegions(Array.from(document.querySelectorAll('[data-hit]')).map(element => {
      const rect = element.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    })));
  });
  $effect(() => {
    const undoVisible = now < snapshot.undoUntil;
    const count = todos.length;
    if (role === 'stack' && ready) { undoVisible; count; arranging; send('stack-layout'); }
  });
  $effect(() => {
    width; height;
    if (role === 'quick') void tick().then(() => hitRegions([{ x: 0, y: 0, width, height }]));
  });
  onMount(() => {
    const disposeApp = trackMemory('App');
    const disposeListeners = ['resize', 'keydown', 'pointerdown'].map(() => trackMemory('documentOrWindowListener'));
    const disposeObserver = trackMemory('ResizeObserver');
    let dispose = () => {};
    let disposed = false;
    void connect(incoming => { snapshot = incoming; ready = true; now = Date.now(); if (role === 'quick' && incoming.selectedId) selected = incoming.selectedId; }, value => { if (mode !== value) error = ""; mode = value as InputMode; if (mode === 'KeyboardActive') void tick().then(() => keyboardRoot?.focus()); }, config => { side = config.side; offset = config.offset; stackIndex = config.stackIndex; stackCount = config.stackCount; stackId = config.stackId; itemHeight = config.itemHeight; workHeight = config.workHeight; viewportTop = hostPlatform === 'darwin' ? config.viewportTop : 0; layoutRevision = config.layoutRevision; }, message => { error = message; }, presentation => { if (role === "quick" && quickTask !== presentation.todoId) error = ""; quickOpen = presentation.quickOpen; quickSource = presentation.sourceIndex; quickTask = presentation.todoId; quiet = presentation.quiet; arranging = !!presentation.arranging; }, pointer => { nativePointer = pointer; }, status => { notificationStatus = status; now = Date.now(); }, value => { settingsState = value; }, open => { settingsOpen = open; }, value => { addSession = value; }, controlVisibility).then(cleanup => { if (disposed) cleanup(); else dispose = cleanup; }).catch(cause => { error = String(cause); });
    const resize = () => { if (role === 'stack' || role === 'quick') { height = innerHeight; width = innerWidth; } };
    window.addEventListener('resize', resize);
    document.addEventListener('keydown', keyboard);
    document.addEventListener('pointerdown', outside);
    const observer = new ResizeObserver(() => { if (canvas) width = canvas.clientWidth; });
    if (canvas) observer.observe(canvas);
    if (role === 'quick' || role === 'add') void tick().then(() => hitRegions([{ x: 0, y: 0, width: innerWidth, height: innerHeight }]));
    return () => { disposed = true; dispose(); observer.disconnect(); window.removeEventListener('resize', resize); document.removeEventListener('keydown', keyboard); document.removeEventListener('pointerdown', outside); disposeApp(); disposeListeners.forEach(off => off()); disposeObserver(); };
  });
</script>

{#snippet card()}
  {#key `${current?.id}-${overflowOpen}`}
    <QuickCard todo={current} overflow={role === 'quick' || overflowOpen ? cardOverflow : []} editing={mode === 'Editing'} busy={pendingCount > 0} {error} {now} onClose={() => closeQuick()} onEdit={startEditing} onSave={(title, description) => finishEditing(true, title, description)} onCancel={() => finishEditing(false)} onComplete={() => current && complete(current.id)} onSnooze={snooze} onPresence={inside => presence('card', inside)} onChoose={todo => { selected = todo.id; quickTask = todo.id; overflowOpen = false; if (native) act({ type: 'select', id: todo.id }); }} />
  {/key}
{/snippet}

{#if role === 'add'}
  <QuickAdd session={addSession} />
{:else if role === 'quick'}
  <div class="native-quick" bind:this={keyboardRoot} tabindex="-1">{@render card()}</div>
{:else if role === 'stack'}
  <div class="native-stack" bind:this={keyboardRoot} tabindex="-1">
    <EdgeStack {now} {todos} {side} {offset} height={stackHeight} {viewportTop} {layoutRevision} {width} {itemHeight} {mode} {selected} {locked} {nativePointer} {arranging} orderError={error} onMove={reorder} onFinish={() => arrange(false)} onShowAll={() => send("show-control")} onSelect={choose} onOpen={open} onComplete={complete} onOverflow={openOverflow} onRegions={receiveRegions} onPresence={inside => presence('source', inside)} onMetric={metric} />
    {#if now < snapshot.undoUntil}<div class="undo-toast" data-sidelet><span>已完成</span><button onclick={() => act({ type: 'undo' })}>撤销</button></div>{/if}
  </div>
{:else if native && snapshot.storage === 'sqlite'}
  <div data-control-root hidden={!controlVisible}>
  {#if settingsOpen}<SettingsPanel externalError={error} settings={settingsState} {notificationStatus} onClose={() => settingsOpen = false} />{/if}
 <div hidden={settingsOpen}>
  <TodoManager onQuickAdd={() => send('quick-add-open')} onSettings={() => { error = ""; settingsOpen = true; send("settings-refresh"); }} {notificationStatus} onNotificationPermission={() => send("notification-permission")} {arranging} onArrange={() => arrange(!arranging)} onMove={reorder} {snapshot} {ready} {error} {side} {offset} {itemHeight} {quiet} {now} onAction={act} onLayout={(type, payload) => send(type, payload)} onQuiet={toggleQuiet} onHide={() => send('hide-control')} />
 </div>
  </div>
{:else}
  <main class="lab">
    <header class="lab-header"><a class="brand" href={native ? '/?view=control' : '/'} aria-label="Sidelet 首页"><span class="brand-mark" aria-hidden="true">s.</span>sidelet</a><span class="phase">PHASE 00 <span>/</span> EDGE WINDOW SPIKE</span><span class="candidate"><span></span>Development Candidate</span></header>
    <section class="intro"><div><p class="eyebrow">GLANCE → HOVER → ACT</p><h1>在边缘，保持可见。</h1><p>任务留在视野里，注意力留在工作中。</p></div><div class="intro-meta"><span>8 条假数据 · 无数据库</span><span>{native ? `${hostPlatform === 'darwin' ? 'macOS' : 'Windows'} 原生窗口控制` : '浏览器交互预览'}</span></div></section>
    <div class="lab-layout">
      <aside class="lab-controls" data-sidelet>
        <h2>验证场景</h2><p class="panel-note">调整边界，检查交互。</p>
        <div class="control-group"><span class="control-label">屏幕边缘</span><div class="segmented"><button class:active={side === 'left'} onclick={() => updateSide('left')}>左侧</button><button class:active={side === 'right'} onclick={() => updateSide('right')}>右侧</button></div></div>
        <label class="control-group"><span class="control-label">中心位置 <output>{Math.round(offset * 100)}%</output></span><input aria-label="Stack 中心位置" type="range" min="0" max="1" step="0.01" bind:value={offset} oninput={() => { if (native) send('offset', { offset }); }} /></label>
        {#if !native}
          <label class="control-group"><span class="control-label">工作区高度 <output>{height}px</output></span><input aria-label="工作区高度" type="range" min="180" max="620" step="10" bind:value={height} /></label>
        {/if}
        <label class="control-group"><span class="control-label">标签高度 <output>{itemHeight}px</output></span><input aria-label="标签高度" type="range" min="40" max="64" step="4" bind:value={itemHeight} oninput={() => { if (native) send('density', { itemHeight }); }} /></label>
        {#if !native}
          <label class="toggle"><input type="checkbox" bind:checked={showRegions} />显示可交互区域</label>
        {/if}
        <div class="mode-status"><span class="control-label">输入状态</span><strong>{mode}</strong><span>{mode === 'Passive' ? '鼠标查看，键盘留给当前应用。' : mode === 'Editing' ? '正在编辑任务。' : '↑ ↓ 选择 · Space 完成 · Esc 退出'}</span></div>
        <button class="primary wide" disabled={quiet} onclick={() => native ? send('keyboard') : void inputMode('KeyboardActive')}>进入键盘操作 <kbd>{shortcutLabel}</kbd></button>
        <div class="secondary-actions"><button onclick={reset}>重置假数据</button><button onclick={toggleQuiet}> {quiet ? '恢复显示' : '安静模式'}</button></div>
        <div class="event-log"><span class="control-label">最近操作</span>{#if !eventLog.length}<p>悬停在右侧标签上开始。</p>{/if}{#each eventLog as entry}<p>{entry}</p>{/each}</div>
        {#if native}<button class="text-button" onclick={() => send('hide-control')}>隐藏面板，观察桌面窗口</button>{/if}
      </aside>
      <section class="preview-panel">
        <div class="preview-toolbar"><span class="monitor-dot"></span><span>{native ? '交互示意 · 原生标签在桌面边缘' : '模拟工作区'}</span><span class="workarea-meta">{Math.round(width)} × {height} logical px</span></div>
        <div class="workarea" bind:this={canvas} bind:clientWidth={width} style:height={`${height}px`} tabindex="-1">
          <div class="workspace-document"><p class="eyebrow">当前工作</p><h2>把注意力留给手头的事。</h2><p>继续输入，Sidelet 在鼠标经过时展开。</p><textarea aria-label="下层应用输入测试" placeholder="在这里输入 service，检查 S / E / Space 是否正常…" rows="3"></textarea><button class="underlying-button" onclick={() => underlyingClicks++}>下层应用按钮 <span>{underlyingClicks} 次点击</span></button><div class="document-lines" aria-hidden="true"><i></i><i></i><i></i></div></div>
          {#if !quiet}
            <EdgeStack {now} {todos} {side} {offset} {height} {width} {itemHeight} {mode} {selected} {locked} onSelect={choose} onOpen={open} onComplete={complete} onOverflow={openOverflow} onRegions={receiveRegions} onPresence={inside => presence('source', inside)} onMetric={metric} />
            {#if showRegions}{#each regions as rect}<div class="region-debug" style:left={`${rect.x}px`} style:top={`${rect.y}px`} style:width={`${rect.width}px`} style:height={`${rect.height}px`}></div>{/each}{/if}
            {#if quickOpen && !native}<div class="positioned-quick" style:left={`${quickBounds.x}px`} style:top={`${quickBounds.y}px`} style:width={`${quickBounds.width}px`} style:height={`${quickBounds.height}px`}>{@render card()}</div>{/if}
          {/if}
          {#if now < snapshot.undoUntil}<div class="undo-toast" data-sidelet><span>已完成「{snapshot.todos.find(t => t.id === snapshot.undoId)?.title}」</span><button onclick={() => act({ type: 'undo' })}>撤销</button></div>{/if}
        </div>
        <div class="preview-foot"><span>{layout.direct} 条直接显示{layout.overflow ? ` · ${layout.overflow} 条收进 +N` : ''}</span><span>Hover 150ms · 收起 500ms · Undo 5s</span></div>
      </section>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <footer class="lab-footer"><span>{native ? '原生验收请按项目文档记录结果。' : '此页面验证交互与布局；原生命中、焦点和资源占用需桌面实测。'}</span><span>CALM BY DEFAULT.</span></footer>
  </main>
{/if}
