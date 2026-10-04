export type InputMode = 'Passive' | 'KeyboardActive' | 'Editing';
export interface Todo {
  id: number; title: string; description: string; dueAt: number;
  completed: boolean; completedAt: number; snoozedUntil: number;
  remind?: boolean; remindAt?: number; reminderSentAt?: number;
  priority?: number; temporary?: boolean; createdAt?: number; updatedAt?: number;
  displayMode?: 'NONE' | 'EDGE'; stackId?: number; sortOrder?: number;
}
export interface EdgeStackState { id: number; displayId: string; side: 'left' | 'right'; offset: number; density: 'compact' | 'normal' | 'relaxed' }
export interface Snapshot { todos: Todo[]; undoId: number; undoUntil: number; selectedId: number; overflowIds?: number[]; storage?: 'memory' | 'sqlite'; stacks?: EdgeStackState[] }
export interface Action { type: string; id?: number; title?: string; description?: string; duration?: string; dueAt?: number; priority?: number; temporary?: boolean; pin?: boolean; stackId?: number; remind?: boolean; requestId?: string; order?: number[]; previousOrder?: number[] }

export function tomorrowMorning(now: number): number {
  const date = new Date(now);
  date.setDate(date.getDate() + 1);
  date.setHours(9, 0, 0, 0);
  return date.getTime();
}

export function initialSnapshot(now = Date.now()): Snapshot {
  const titles = ['准备周会材料', '回复客户的产品反馈', '检查新版本构建', '整理本周设计记录', '确认接口联调时间', '阅读 20 分钟', '预约周末体检', '提交发布前检查清单'];
  const due = new Date(now); due.setHours(16, 0, 0, 0);
  return { storage: 'memory', todos: titles.map((title, i) => ({ id: i + 1, title, description: i === 0 ? '整理最新数据\n确认演示流程\n和团队对齐本周进度' : '这是用于验证桌面交互的假数据。\n所有修改仅保留在本次运行中。', dueAt: i < 3 ? due.getTime() + i * 3600000 : 0, completed: false, completedAt: 0, snoozedUntil: 0 })), undoId: 0, undoUntil: 0, selectedId: 0 };
}

// Browser-only counterpart of the native in-memory fixture reducer.
export function reduce(state: Snapshot, action: Action, now = Date.now()): Snapshot {
  if (action.type === 'reset') return initialSnapshot(now);
  // Svelte state can be a Proxy; copy the fixture fields rather than structuredClone it.
  const next: Snapshot = { ...state, todos: state.todos.map(todo => ({ ...todo })) };
  const todo = next.todos.find(t => t.id === action.id);
  if (action.type === 'select') next.selectedId = action.id ?? 0;
  if (action.type === 'complete' && todo && !todo.completed) {
    todo.completed = true; todo.completedAt = now;
    next.undoId = todo.id; next.undoUntil = now + 5000;
  }
  if (action.type === 'undo' && next.undoId && now <= next.undoUntil) {
    const undone = next.todos.find(t => t.id === next.undoId);
    if (undone) { undone.completed = false; undone.completedAt = 0; }
    next.undoId = 0; next.undoUntil = 0;
  }
  if (action.type === 'snooze' && todo) todo.snoozedUntil = action.duration === 'tomorrow' ? tomorrowMorning(now) : now + (action.duration === '1h' ? 3600000 : 1800000);
  if (action.type === 'edit' && todo && action.title?.trim()) {
    todo.title = action.title.trim(); todo.description = action.description ?? todo.description;
  }
  return next;
}

export function visibleTodos(state: Snapshot, now: number) {
  return state.todos.filter(t => (state.storage !== 'sqlite' || t.displayMode === 'EDGE') && (!t.completed || now < t.completedAt + 800) && t.snoozedUntil <= now);
}

export function nextDeadline(state: Snapshot, now: number) {
  const times = [state.undoUntil, ...state.todos.flatMap(t => [t.completed ? t.completedAt + 800 : 0, t.snoozedUntil, !t.completed && t.dueAt ? t.dueAt - 30 * 60000 : 0, !t.completed && t.dueAt ? t.dueAt : 0])].filter(t => t > now);
  return times.length ? Math.min(...times) : undefined;
}

export function dueState(todo: Todo, now: number): 'none' | 'soon' | 'overdue' {
  if (!todo.dueAt || todo.completed || todo.snoozedUntil > now) return 'none';
  if (todo.dueAt <= now) return 'overdue';
  return todo.dueAt - now <= 30 * 60000 ? 'soon' : 'none';
}
export function dueLabel(todo: Todo, now: number): string {
  return ({ none: '', soon: '即将到期', overdue: '已逾期' })[dueState(todo, now)];
}
