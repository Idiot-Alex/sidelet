'use strict';
const byId = id => document.getElementById(id);
let state = { revision: -1, tasks: [] }, filter = 'pending', edit = null, busy = false;
const rows = new Map();
const priorities = { 0: '普通', 2: '重要', 3: '紧急' };
const element = (tag, className, text) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};
function button(text, label, action, className = '') {
  const node = element('button', className, text);
  node.type = 'button';
  node.setAttribute('aria-label', label);
  node.disabled = busy;
  node.addEventListener('click', action);
  return node;
}
function prioritySelect(value, label) {
  const select = element('select');
  select.setAttribute('aria-label', label);
  for (const [key, title] of Object.entries(priorities)) {
    const option = element('option', '', title);
    option.value = key;
    select.append(option);
  }
  select.value = String(value);
  return select;
}
function apply(snapshot) {
  if (snapshot.revision < state.revision) return;
  state = snapshot;
  render();
}
async function call(method, ...args) {
  if (busy) return null;
  busy = true;
  byId('add-button').disabled = true;
  byId('status').textContent = '';
  try {
    return await mygo.call(`TasksService.${method}`, ...args);
  } catch (error) {
    byId('status').textContent = error.message;
    // Refresh the authoritative version, retaining the user's editor draft.
    try { apply(await mygo.call('TasksService.List')); } catch (_) {}
    return null;
  } finally {
    busy = false;
    byId('add-button').disabled = false;
    for (const node of document.querySelectorAll('.task-row button')) node.disabled = false;
  }
}
function taskRow(task) {
  const row = element('article', `task-row${task.done ? ' completed' : ''}`);
  row.dataset.id = task.id;
  const check = button('', `${task.done ? '恢复' : '完成'} ${task.title}`, async () => {
    const result = await call('SetDone', task.id, !task.done, task.version);
    if (result) apply(result);
  }, `task-check${task.done ? ' done' : ''}`);
  if (task.done) check.innerHTML = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="m3 8 3 3 7-7" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  check.setAttribute('aria-pressed', String(task.done));
  const body = element('div', 'task-body');
  body.append(element('h2', 'task-title', task.title));
  const meta = element('div', 'task-meta');
  meta.append(element('span', `priority ${task.priority === 2 ? 'important' : task.priority === 3 ? 'urgent' : ''}`, priorities[task.priority]));
  meta.append(element('span', 'note', task.note));
  body.append(meta);
  const actions = element('div', 'task-actions');
  actions.append(button('编辑', `编辑 ${task.title}`, () => startEdit(task)));
  if (!task.done) actions.append(button('打开卡片', `打开卡片 ${task.title}`, () => call('Open', task.id)));
  row.append(check, body, actions);
  return row;
}
function startEdit(task) {
  if (busy) return;
  edit = { id: task.id, version: task.version };
  const row = element('article', 'task-row');
  row.dataset.id = task.id;
  const form = element('form', 'editor');
  const input = element('input');
  input.value = task.title;
  input.maxLength = 120;
  input.required = true;
  input.setAttribute('aria-label', '编辑任务标题');
  const select = prioritySelect(task.priority, '编辑任务重要程度');
  const actions = element('div', 'editor-actions');
  actions.append(button('取消', '取消编辑', cancelEdit));
  const save = element('button', 'primary', '保存');
  save.type = 'submit';
  actions.append(save);
  form.append(input, select, actions);
  row.append(form);
  edit.row = row;
  form.addEventListener('submit', async event => {
    event.preventDefault();
    const editing = edit;
    if (!editing || busy) return;
    const result = await call('Update', editing.id, input.value, Number(select.value), editing.version);
    if (result) { edit = null; apply(result); }
  });
  form.addEventListener('keydown', event => {
    if (event.isComposing) return;
    if (event.key === 'Escape') { event.preventDefault(); cancelEdit(); }
  });
  render();
  input.focus();
  input.select();
}
function cancelEdit() { if (busy) return; edit = null; render(); }
function render() {
  const pending = state.tasks.filter(task => !task.done).length;
  byId('summary').textContent = `${pending} 项待办`;
  byId('pending-count').textContent = pending;
  byId('done-count').textContent = state.tasks.length - pending;
  for (const node of document.querySelectorAll('[data-filter]')) node.setAttribute('aria-pressed', String(node.dataset.filter === filter));
  const list = byId('tasks');
  const visible = state.tasks.filter(task => filter === 'done' ? task.done : !task.done);
  // Keep the live editor DOM untouched during events, including IME composition.
  const wanted = new Set(visible.map(task => task.id));
  if (edit && !wanted.has(edit.id)) { edit = null; byId('status').textContent = '任务状态已更新，编辑已关闭。'; }
  for (const child of [...list.children]) if (!wanted.has(Number(child.dataset.id))) child.remove();
  visible.forEach((task, index) => {
    let row;
    if (edit?.id === task.id) row = edit.row;
    else {
      const cached = rows.get(task.id);
      row = cached?.version === task.version ? cached.row : taskRow(task);
      rows.set(task.id, { version: task.version, row });
    }
    if (list.children[index] !== row) {
      const old = list.children[index];
      if (old?.dataset.id === String(task.id)) old.replaceWith(row);
      else list.insertBefore(row, old || null);
    }
  });
  byId('empty').hidden = visible.length !== 0;
}
byId('add-form').addEventListener('submit', async event => {
  event.preventDefault();
  const result = await call('Add', byId('new-title').value, Number(byId('new-priority').value));
  if (result) { byId('new-title').value = ''; apply(result); byId('new-title').focus(); }
});
for (const node of document.querySelectorAll('[data-filter]')) node.addEventListener('click', () => {
  if (busy) return;
  filter = node.dataset.filter;
  edit = null;
  byId('status').textContent = '';
  render();
});
if (window.mygo) {
  mygo.on('lab:tasks-changed', apply);
  mygo.call('TasksService.List').then(apply).catch(error => { byId('status').textContent = error.message; });
} else byId('status').textContent = '请在 Sidelet 实验应用中打开任务窗口。';
