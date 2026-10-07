<script lang="ts">
  import { tick } from 'svelte';
  import { normalizePriority, priorityLabel, priorityOptions } from '../lib/priority';
  import { dueLabel, type Action, type Snapshot, type Todo } from '../lib/model';
  import type { NotificationStatus } from '../lib/bridge';
  import AppHeader from './AppHeader.svelte';
  import Icon from './Icon.svelte';
  import IconText from './IconText.svelte';
  import TaskOrder from './TaskOrder.svelte';
  import type { DropTarget } from '../lib/reorder';
  let { onQuickAdd, onSettings, notificationStatus, onNotificationPermission, snapshot, ready, error, side, offset, itemHeight, quiet, now, onAction, onLayout, onQuiet, onHide, arranging, onArrange, onMove }:
    { onQuickAdd: () => void; onSettings: () => void; notificationStatus: NotificationStatus; onNotificationPermission: () => void; snapshot: Snapshot; ready: boolean; error: string; side: 'left' | 'right'; offset: number; itemHeight: number; quiet: boolean; now: number; onAction: (action: Action) => Promise<boolean>; onLayout: (type: string, payload: Record<string, unknown>) => void; onQuiet: () => void; onHide: () => void; arranging: boolean; onArrange: () => void; onMove: (id: number, target: DropTarget) => Promise<boolean> } = $props();
  let formOptions = $state(false);
  let filter = $state<'pending' | 'completed' | 'all'>('pending');
  let editing = $state(0);
  let title = $state('');
  let description = $state('');
  let due = $state('');
  let originalDue = 0;
  let originalDueText = '';
  let priority = $state(0);
  let temporary = $state(false);
  let pin = $state(false);
  let remind = $state(false);
  $effect(() => { if (!due) remind = false; });
  let busy = $state(false);
  let deleteID = $state(0);
  let moreID = $state(0);
  let moreTrigger: HTMLButtonElement | undefined;
  let formError = $state('');
  let titleInput = $state<HTMLInputElement>();
  const pending = $derived(snapshot.todos.filter(todo => !todo.completed));
  const completed = $derived(snapshot.todos.filter(todo => todo.completed));
  const listed = $derived(filter === 'all' ? snapshot.todos : filter === 'completed' ? completed : pending);
  $effect(() => {
    if (moreID && (arranging || !listed.some(todo => todo.id === moreID))) closeMore();
  });

  function closeMore(restoreFocus = false) {
    moreID = 0; deleteID = 0;
    if (restoreFocus && moreTrigger?.isConnected) moreTrigger.focus();
  }
  async function toggleMore(id: number, trigger: HTMLButtonElement) {
    if (moreID === id) { closeMore(); return; }
    moreTrigger = trigger; moreID = id; deleteID = 0;
    await tick();
    if (moreID === id) document.querySelector<HTMLButtonElement>(`[data-task-more="${id}"] .request-delete`)?.focus();
  }
  function dismissMore(event: Event) {
    if (!moreID || !(event.target instanceof Element)) return;
    if (event.type === 'focusin' && (event.target === document.body || event.target === document.documentElement)) return;
    if (event.target.closest('[data-task-more]')?.getAttribute('data-task-more') !== String(moreID)) closeMore();
  }
  function moreKey(event: KeyboardEvent) {
    if (event.isComposing || !moreID) return;
    if (event.key === 'Escape' && !event.isComposing && moreID) {
      event.preventDefault(); event.stopPropagation(); closeMore(true);
      return;
    }
    const panel = document.getElementById(`task-more-${moreID}`);
    if (!panel?.contains(document.activeElement)) return;
    if (['Tab', 'ArrowDown', 'ArrowUp'].includes(event.key)) {
      const buttons = Array.from(panel.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));
      const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
      const step = event.key === 'ArrowUp' || event.key === 'Tab' && event.shiftKey ? -1 : 1;
      const next = buttons[index + step];
      if (next) { event.preventDefault(); next.focus(); }
      else if (event.key !== 'Tab') { event.preventDefault(); buttons[step > 0 ? 0 : buttons.length - 1]?.focus(); }
      else if (step < 0) { event.preventDefault(); closeMore(true); }
    }
  }
  async function requestDelete(id: number) {
    deleteID = id;
    await tick();
    document.querySelector<HTMLButtonElement>(`[data-task-more="${id}"] .cancel-delete`)?.focus();
  }
  async function cancelDelete(id: number) {
    deleteID = 0;
    await tick();
    document.querySelector<HTMLButtonElement>(`[data-task-more="${id}"] .request-delete`)?.focus();
  }

  function localTime(value: number) {
    if (!value) return '';
    const date = new Date(value);
    const pad = (number: number) => String(number).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
  }
  function clearForm() {
    closeMore();
    editing = 0; title = ''; description = ''; due = ''; priority = 0; temporary = false; pin = false; remind = false;
    originalDue = 0; originalDueText = ''; formError = ''; formOptions = false;
  }
  function edit(todo: Todo) {
    closeMore();
    formOptions = true; editing = todo.id; title = todo.title; description = todo.description;
    due = localTime(todo.dueAt); originalDue = todo.dueAt; originalDueText = due;
    remind = todo.remind ?? false; priority = normalizePriority(todo.priority); temporary = todo.temporary ?? false; formError = ''; titleInput?.focus();
  }
  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (busy || !ready || arranging || !title.trim()) return;
    const deadline = editing && due === originalDueText ? originalDue : due ? new Date(due).getTime() : 0;
    if (!Number.isFinite(deadline)) { formError = '请选择有效的截止时间。'; return; }
    busy = true; formError = '';
    try {
      const ok = await onAction({ type: editing ? 'edit' : 'create', id: editing, title, description, dueAt: deadline, priority, temporary, pin, remind });
      if (ok) clearForm();
    } finally { busy = false; }
  }
  async function action(value: Action) {
    if (busy || !ready) return;
    busy = true;
    try { if (await onAction(value)) { if (value.type === 'delete') { if (editing === value.id) clearForm(); closeMore(); } } }
    finally { busy = false; }
  }
</script>

<svelte:window onpointerdown={dismissMore} onfocusin={dismissMore} onkeydown={moreKey} onblur={() => closeMore()} />

<main class="manager" data-sidelet>
  <AppHeader title="我的任务" count={pending.length}>
    <button class="toolbar-button" onclick={onQuickAdd}><IconText name="plus">快速添加</IconText></button>
    <button class="toolbar-button" disabled={!ready || quiet || busy} aria-pressed={arranging} onclick={onArrange}><IconText name="layout">{arranging ? '完成整理' : '整理桌面'}</IconText></button>
    <button class="toolbar-button" class:engaged={quiet} aria-pressed={quiet} onclick={onQuiet}><IconText name="moon">{quiet ? '恢复显示' : '安静模式'}</IconText></button>
    <span class="toolbar-divider"></span>
    <button class="icon-button" aria-label="设置" title="设置 · ⌘," disabled={busy || arranging} onclick={onSettings}><Icon name="settings" size={18} /></button>
    <button class="icon-button" aria-label="隐藏窗口" title="隐藏窗口" onclick={onHide}><Icon name="hide" size={18} /></button>
  </AppHeader>
  {#if error || formError}<p class="error-message" role="alert">{formError || error}</p>{/if}
  {#if notificationStatus.message && (remind || snapshot.todos.some(todo => todo.remind))}
    <div class="notification-status" role="status"><span>{notificationStatus.message}</span>{#if notificationStatus.authorization !== 'unsupported'}<button onclick={onNotificationPermission}>{notificationStatus.authorization === 'notDetermined' ? '开启系统通知' : '检查通知权限'}</button>{/if}</div>
  {/if}
  {#if now < snapshot.undoUntil}<div class="undo"><span>已完成「{snapshot.todos.find(todo => todo.id === snapshot.undoId)?.title}」</span><button disabled={busy} onclick={() => action({ type: 'undo' })}>撤销完成</button></div>{/if}
  <div class="columns">
    <section class="task-list" aria-label="任务列表">
      {#if arranging}
        <h2>桌面任务顺序</h2><p class="hint">拖动手柄调整顺序，松手自动保存。Option + ↑ / ↓ 也可调整。已完成和暂时隐藏的任务保留原位。</p>
        {#each snapshot.stacks ?? [] as stack (stack.id)}
          {@const members = snapshot.todos.filter(todo => todo.stackId === stack.id && !todo.completed && todo.snoozedUntil <= now)}
          <h3 class="stack-heading">{stack.side === 'left' ? '左侧' : '右侧'}桌面 · {members.length} 项</h3>
          {#if members.length}<TaskOrder todos={members} {onMove} />{:else}<p class="hint">暂无可整理的任务，请先固定任务到桌面。</p>{/if}
        {/each}
      {:else}
      <nav aria-label="任务筛选"><button class:active={filter === 'pending'} aria-pressed={filter === 'pending'} onclick={() => filter = 'pending'}>待办 {pending.length}</button><button class:active={filter === 'completed'} aria-pressed={filter === 'completed'} onclick={() => filter = 'completed'}>已完成 {completed.length}</button><button class:active={filter === 'all'} aria-pressed={filter === 'all'} onclick={() => filter = 'all'}>全部 {snapshot.todos.length}</button><span class="local-note"><i></i>自动保存在本机</span></nav>
      {#if !ready}<p class="empty">正在读取任务…</p>
      {:else if !listed.length}<div class="empty"><div class="empty-art" aria-hidden="true"><span></span><span></span><span></span><Icon name="check" size={24} /></div><h2>{filter === 'completed' ? '还没有已完成的任务。' : snapshot.todos.length ? '这里暂时没有任务。' : '从一件小事开始'}</h2><p>{snapshot.todos.length ? '可以切换筛选查看其他任务。' : '写下接下来想做的事，让它留在视野里。'}</p>{#if !snapshot.todos.length}<button class="empty-action" onclick={() => titleInput?.focus()}><IconText name="plus" gap={5}>写下第一件事</IconText></button>{/if}</div>{/if}
      {#each listed as item (item.id)}
        <article class:done={item.completed} class:selected={editing === item.id} class:menu-open={moreID === item.id} aria-label={`任务：${item.title}`}>
          <div class="task-row-main">
          <button class="complete" aria-label={item.completed ? `恢复${item.title}` : `完成${item.title}`} disabled={busy} onclick={() => action({ type: item.completed ? 'reopen' : 'complete', id: item.id })}>{#if item.completed}<Icon name="check" size={13} />{/if}</button>
          <button class="task-title" title={item.title} aria-pressed={editing === item.id} disabled={busy} onclick={() => edit(item)}>{item.title}</button>
          <div class="task-status">
            {#if item.priority && !item.completed}<span class="priority" class:urgent={item.priority >= 3}>{priorityLabel(item.priority)}</span>{/if}
            {#if item.dueAt}<span class="deadline" class:late={!item.completed && item.dueAt < now}><IconText name="clock" size={12} gap={4}>{dueLabel(item, now) ? dueLabel(item, now) + ' · ' : ''}{new Date(item.dueAt).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })}</IconText></span>{/if}
          </div>
          <div class="row-actions">
            <button aria-label={`编辑${item.title}`} title="编辑任务" disabled={busy} onclick={() => edit(item)}><Icon name="edit" size={15} /></button>
            <button class:pinned={item.displayMode === 'EDGE'} aria-label={`${item.displayMode === 'EDGE' ? '取消桌面固定' : '固定到桌面'}：${item.title}`} title={item.displayMode === 'EDGE' ? '取消桌面固定' : '固定到桌面'} aria-pressed={item.displayMode === 'EDGE'} disabled={busy} onclick={() => action({ type: item.displayMode === 'EDGE' ? 'unpin' : 'pin', id: item.id })}><Icon name="pin" size={15} /></button>
            <button data-task-more={item.id} aria-label={`更多操作：${item.title}`} title="更多操作" aria-expanded={moreID === item.id} aria-controls={`task-more-${item.id}`} disabled={busy} onclick={event => toggleMore(item.id, event.currentTarget)}><Icon name="more" size={17} /></button>
          </div>
          </div>
          {#if item.description || item.remind && !item.completed || item.displayMode === 'EDGE' || item.temporary || item.snoozedUntil > now}
            <div class="task-secondary">
              {#if item.description}<p class="description">{item.description}</p>{/if}
              <div class="task-meta">{#if item.displayMode === 'EDGE'}<span class="pinned"><IconText name="pin" size={11} gap={3}>桌面</IconText></span>{/if}{#if item.remind && !item.completed}<span>{item.reminderSentAt ? '提醒已交给系统' : '系统提醒已开启'}</span>{/if}{#if item.temporary}<span>临时</span>{/if}{#if item.snoozedUntil > now}<span>暂时隐藏至 {new Date(item.snoozedUntil).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })}</span><button disabled={busy} onclick={() => action({ type: 'unsnooze', id: item.id })}>现在恢复</button>{/if}</div>
            </div>
          {/if}
          {#if moreID === item.id}
            <div class="row-more" id={`task-more-${item.id}`} data-task-more={item.id} role="group" aria-label={`更多任务操作：${item.title}`}>
              {#if deleteID === item.id}<span class="delete-prompt">删除此任务？</span><div class="delete-actions"><button class="cancel-delete" disabled={busy} onclick={() => cancelDelete(item.id)}>取消</button><button class="danger" disabled={busy} onclick={() => action({ type: 'delete', id: item.id })}>确认删除</button></div>{:else}<button class="request-delete danger" disabled={busy} onclick={() => requestDelete(item.id)}><IconText name="trash" size={14}>删除任务</IconText></button>{/if}
            </div>
          {/if}
        </article>
      {/each}
      {/if}
    </section>
    <aside aria-label="任务编辑器">
      <form onsubmit={submit}>
        <div class="form-heading"><span class="form-symbol"><Icon name={editing ? 'list' : 'plus'} size={18} /></span><h2>{editing ? '编辑任务' : '新建任务'}</h2></div>
        <label>标题<input aria-label={editing ? '编辑任务标题' : '新任务标题'} bind:this={titleInput} bind:value={title} maxlength="500" required disabled={busy || !ready || arranging} placeholder="接下来想做什么？" /></label>
        <label>备注<textarea aria-label="任务备注" bind:value={description} rows="3" maxlength="16000" disabled={busy || !ready || arranging} placeholder="补充要点，或留空"></textarea></label>
        {#if !editing}<label class="check"><input type="checkbox" bind:checked={pin} disabled={busy || !ready || arranging} /><IconText name="pin" size={14} gap={7}>固定到桌面</IconText></label>{/if}
        <details class="form-options" bind:open={formOptions}><summary>截止时间与更多选项</summary><div class="options-body">
        <label>截止时间<input aria-label="任务截止时间" type="datetime-local" bind:value={due} disabled={busy || !ready || arranging} /></label>
        <label class="check"><input type="checkbox" aria-label="提醒我" bind:checked={remind} disabled={busy || !ready || arranging || !due} />提醒我</label>
        <p class="hint">{remind ? '到截止时间发送系统通知；Sidelet 需在后台运行。' : '默认只显示到期状态；设置截止时间后可开启通知。'}</p>
        <label>重要程度<select aria-label="任务重要程度" bind:value={priority} disabled={busy || !ready || arranging}>{#each priorityOptions as option}<option value={option.value}>{option.label}</option>{/each}</select></label>
        <label class="check"><input type="checkbox" bind:checked={temporary} disabled={busy || !ready || arranging} />临时任务</label>
        {#if temporary}<p class="hint">完成后保留 5 秒撤销时间，随后自动移除。</p>{/if}
        </div></details>
        <div class="form-actions"><button class="primary" type="submit" disabled={busy || !ready || arranging || !title.trim()}>{busy ? '保存中…' : editing ? '保存修改' : '添加任务'}</button>{#if editing}<button type="button" onclick={clearForm} disabled={busy}>取消编辑</button>{/if}</div>
      </form>
        <details class="layout-controls"><summary><IconText name="layout" gap={8}>桌面位置</IconText><span>{side === 'left' ? '左侧' : '右侧'}</span><Icon name="chevron" size={14} /></summary><div class="options-body">
        <label>屏幕边缘<select aria-label="桌面边缘" value={side} onchange={event => onLayout('side', { side: event.currentTarget.value })} disabled={!ready || arranging}><option value="left">左侧</option><option value="right">右侧</option></select></label>
        <label>中心位置 <output>{Math.round(offset * 100)}%</output><input aria-label="桌面中心位置" type="range" min="0" max="1" step="0.01" value={offset} onchange={event => onLayout('offset', { offset: Number(event.currentTarget.value) })} disabled={!ready || arranging} /></label>
        <label>标签密度<select aria-label="桌面标签密度" value={itemHeight} onchange={event => onLayout('density', { itemHeight: Number(event.currentTarget.value) })} disabled={!ready || arranging}><option value={40}>紧凑</option><option value={44}>标准</option><option value={56}>宽松</option></select></label>
      </div></details>
    </aside>
  </div>
</main>

<style>
  .manager { min-height:100vh; padding:0 32px 36px; background:var(--app-bg); color:var(--ink); }
  .local-note { display:flex; align-items:center; gap:6px; margin-left:auto; white-space:nowrap; color:var(--subtle); font-size:11px; }
  .local-note i { width:5px; height:5px; border-radius:50%; background:var(--accent); }
  .columns { display:grid; grid-template-columns:minmax(0,1fr) 296px; gap:24px; align-items:start; margin-top:20px; }
  .task-list { min-width:0; border:1px solid var(--line); background:var(--surface); border-radius:var(--radius); overflow:hidden; padding:16px 20px 12px; min-height:400px; }
  aside { min-width:0; }
  form { padding:20px; border:1px solid var(--line); border-radius:var(--radius); background:var(--surface); }
  .form-heading { display:flex; gap:9px; align-items:center; margin-bottom:22px; }
  .form-symbol { display:grid; place-items:center; color:var(--accent); }
  h2 { font-family:var(--font-heading); font-size:16px; line-height:1.375; font-weight:600; margin:0; }
  label { display:block; font-size:12px; margin-bottom:16px; color:var(--muted); }
  input:not([type=checkbox]), textarea, select { display:block; width:100%; margin-top:7px; font:13px var(--font-ui); color:var(--ink); background:var(--field); padding:10px 11px; border:1px solid var(--line); border-radius:var(--control-radius); min-width:0; }
  textarea { resize:vertical; line-height:1.6; min-height:76px; }
  input[type=datetime-local] { font-size:12px; padding:9px 5px; }
  input[type=range] { padding:0; border:0; background:transparent; margin-top:12px; }
  .check { display:flex; gap:7px; align-items:center; color:var(--ink); font-size:12px; }
  .check input { width:15px; height:15px; margin:0 2px 0 0; }
  .hint { font-size:12px; line-height:1.7; color:var(--muted); margin:0 0 14px; }
  .form-options { border-top:1px solid var(--line); margin-top:2px; }
  summary { cursor:pointer; color:var(--muted); font-size:12px; padding:15px 0; user-select:none; }
  summary::marker { color:var(--subtle); font-size:10px; }
  .options-body { padding-top:3px; }
  .layout-controls { margin-top:14px; border:1px solid var(--line); border-radius:var(--radius); padding:0 16px; }
  .layout-controls summary { display:flex; gap:8px; align-items:center; }
  .layout-controls summary span { margin-left:auto; font-size:11px; }
  .layout-controls[open] summary > :global(svg) { transform:rotate(180deg); }
  output { float:right; font-size:11px; }
  button { font:12px var(--font-ui); color:var(--muted); background:transparent; border:1px solid var(--line); padding:7px 10px; border-radius:var(--control-radius); cursor:pointer; }
  button:hover:not(:disabled) { background:var(--surface-alt); color:var(--ink); }
  button:disabled { opacity:.45; cursor:default; }
  .toolbar-button,.icon-button { display:inline-flex; align-items:center; justify-content:center; gap:6px; border-color:transparent; height:34px; }
  .icon-button { width:34px; padding:0; }
  .toolbar-divider { width:1px; height:18px; background:var(--line); margin:0 5px; }
  .engaged { color:var(--accent); background:var(--accent-soft); }
  .primary { background:var(--accent); border-color:var(--accent); color:var(--on-accent); font-weight:550; }
  .primary:hover:not(:disabled) { background:var(--accent-hover); border-color:var(--accent-hover); color:var(--on-accent); }
  .form-actions { display:flex; gap:8px; }.form-actions button { flex:1; padding:10px 8px; }
  nav { display:flex; gap:4px; padding-bottom:15px; border-bottom:1px solid var(--line); }
  nav button { border:0; padding:7px 12px; font-variant-numeric:tabular-nums; }
  nav .active { background:var(--accent-soft); color:var(--accent); font-weight:550; }
  article { padding:12px 10px; margin:0 -10px; border-bottom:1px solid var(--line); border-radius:6px; }
  article:last-child { border-bottom:0; }
  article:hover,article:focus-within,article.menu-open { background:var(--surface-alt); }
  article.selected { background:var(--accent-soft); box-shadow:inset 2px 0 var(--accent); }
  .task-row-main { display:grid; grid-template-columns:21px minmax(0,1fr) auto auto; align-items:center; gap:10px; min-height:28px; }
  .complete { display:grid; place-items:center; width:20px; height:20px; padding:0; border-radius:50%; border-color:var(--subtle); }
  .complete:hover:not(:disabled) { border-color:var(--accent); background:var(--accent-soft); }
  .done .complete { background:var(--accent); color:var(--on-accent); border-color:var(--accent); }
  .task-title { min-width:0; border:0; padding:2px 0; text-align:left; font-size:14px; line-height:1.5; font-weight:550; color:var(--ink); overflow-wrap:anywhere; display:-webkit-box; line-clamp:2; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
  .task-title:hover:not(:disabled) { background:transparent; color:var(--accent); }
  .done .task-title { color:var(--subtle); text-decoration:line-through; }
  .task-status { display:flex; flex-wrap:wrap; align-items:center; justify-content:flex-end; gap:5px 8px; font-size:11px; color:var(--muted); }
  .deadline { white-space:nowrap; font-variant-numeric:tabular-nums; }
  .task-secondary { margin:3px 0 0 31px; }
  .description { font-size:12px; line-height:1.5; color:var(--muted); white-space:pre-wrap; overflow-wrap:anywhere; margin:0; display:-webkit-box; line-clamp:2; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
  .task-meta { display:flex; flex-wrap:wrap; align-items:center; gap:5px 10px; font-size:11px; line-height:1.5; color:var(--muted); }
  .description + .task-meta:not(:empty) { margin-top:3px; }
  .task-meta:empty { display:none; }.task-meta button { font-size:11px; padding:2px 6px; }
  .pinned { color:var(--accent); }
  .priority { background:var(--warning-soft); color:var(--warning); border-radius:4px; padding:2px 5px; line-height:1.4; white-space:nowrap; }
  .priority.urgent { background:var(--danger-soft); color:var(--danger); }.late { color:var(--danger); }
  .row-actions { display:flex; align-items:center; gap:2px; opacity:0; }
  article:hover .row-actions,article:focus-within .row-actions,article.menu-open .row-actions { opacity:1; }
  .row-actions button { display:grid; place-items:center; width:28px; height:28px; padding:0; border-color:transparent; }
  .row-actions button[aria-expanded=true] { background:var(--accent-soft); color:var(--accent); }
  .row-more { width:220px; max-width:calc(100% - 31px); margin:7px 0 0 auto; padding:6px; background:var(--surface); border:1px solid var(--line); border-radius:6px; }
  .row-more .request-delete { display:flex; align-items:center; width:100%; padding:7px 8px; border-color:transparent; text-align:left; }
  .danger { color:var(--danger); }.row-more .danger:hover:not(:disabled) { color:var(--danger); background:var(--danger-soft); }
  .delete-prompt { display:block; padding:5px 8px 9px; font-size:12px; color:var(--ink); }
  .delete-actions { display:flex; justify-content:flex-end; gap:6px; }.delete-actions button { padding:6px 9px; }
  article button:focus-visible { outline:2px solid var(--accent); outline-offset:2px; }
  @media(hover:none) { .row-actions { opacity:1; } }
  @media(min-width:651px) and (max-width:850px), (max-width:480px) {
    .task-row-main { grid-template-columns:21px minmax(0,1fr) auto; }
    .task-status { grid-column:2; grid-row:2; justify-content:flex-start; }
    .task-status:empty { display:none; }.row-actions { grid-column:3; grid-row:1; }
  }
  .empty { padding:50px 10px; text-align:center; color:var(--muted); }
  .empty h2 { font-size:21px; color:var(--ink); margin:22px 0 10px; }.empty p { font-size:12px; line-height:1.8; }
  .empty-action { display:inline-flex; align-items:center; gap:5px; margin-top:12px; color:var(--accent); border-color:transparent; }
  .empty-art { position:relative; width:78px; margin:0 auto; padding:11px 13px; border:1px solid var(--line); border-radius:12px; background:var(--field); text-align:left; }
  .empty-art span { display:block; height:4px; width:34px; margin:9px 0; border-radius:3px; background:var(--line); }.empty-art span:nth-child(2) { width:24px; }.empty-art span:nth-child(3) { width:30px; }
  .empty-art :global(svg) { position:absolute; right:-9px; bottom:-8px; width:30px; height:30px; padding:6px; border-radius:50%; color:var(--accent); background:var(--accent-soft); border:2px solid var(--surface); }
  .error-message { padding:12px 16px; background:var(--danger-soft); color:var(--danger); font-size:12px; border-radius:var(--control-radius); }
  .undo,.notification-status { display:flex; align-items:center; gap:12px; padding:12px 16px; border-radius:var(--control-radius); font-size:12px; margin-bottom:18px; background:var(--accent-soft); color:var(--accent); }
  .undo span,.notification-status span { flex:1; overflow-wrap:anywhere; }
  .notification-status { background:var(--warning-soft); color:var(--warning); line-height:1.6; }
  .stack-heading { margin:24px 0 12px; color:var(--muted); font-size:12px; font-weight:500; }
  .task-list > h2 { margin:6px 0 12px; }
  @media(max-width:900px) { .manager { padding:0 24px 30px; }.columns { grid-template-columns:minmax(0,1fr) 270px; gap:18px; }.task-list { padding:14px 16px; }form { padding:17px; }.local-note { display:none; } }
  @media(max-width:650px) { .columns { grid-template-columns:1fr; }.toolbar-button { font-size:0; gap:0; }.toolbar-button :global(.icon-text) { gap:0; }.manager { padding:0 16px 24px; } }
  select { appearance:none; -webkit-appearance:none; min-height:36px; padding-right:30px; background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath d='m3 4.5 3 3 3-3' fill='none' stroke='%23839088' stroke-width='1.4' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E"); background-position:right 10px center; background-repeat:no-repeat; }
</style>
