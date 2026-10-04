import { describe, expect, it } from 'vitest';
import { dueState, initialSnapshot, nextDeadline, reduce, tomorrowMorning, visibleTodos } from './model';

describe('Spike timing and fixture lifecycle', () => {
  const now = new Date(2026, 9, 4, 23, 40).getTime();
  it('snoozes to the next local calendar day at 09:00 without changing DueAt or order', () => {
    const before = initialSnapshot(now);
    const after = reduce(before, { type: 'snooze', id: 1, duration: 'tomorrow' }, now);
    expect(after.todos[0].snoozedUntil).toBe(new Date(2026, 9, 5, 9).getTime());
    expect(tomorrowMorning(now)).not.toBe(now + 86400000);
    expect(after.todos[0].dueAt).toBe(before.todos[0].dueAt);
    expect(after.todos.map(t => t.id)).toEqual(before.todos.map(t => t.id));
    expect(visibleTodos(after, now).map(t => t.id)).not.toContain(1);
    expect(visibleTodos(after, after.todos[0].snoozedUntil)[0].id).toBe(1);
  });
  it('retains completion feedback for 800ms and Undo for five seconds', () => {
    const completed = reduce(initialSnapshot(now), { type: 'complete', id: 1 }, now);
    expect(visibleTodos(completed, now + 799).map(t => t.id)).toContain(1);
    expect(visibleTodos(completed, now + 800).map(t => t.id)).not.toContain(1);
    expect(nextDeadline(completed, now)).toBe(now + 800);
    expect(nextDeadline(completed, now + 800)).toBe(now + 5000);
    expect(reduce(completed, { type: 'undo' }, now + 4999).todos[0].completed).toBe(false);
    expect(reduce(completed, { type: 'undo' }, now + 5001).todos[0].completed).toBe(true);
  });
  it('does not need an idle timer for untouched fixtures', () => {
    expect(nextDeadline(initialSnapshot(now), now)).toBeUndefined();
  });
  it('refreshes an upcoming deadline once without recurring idle checks', () => {
    const state = initialSnapshot(now);
    state.todos[0].dueAt = now + 2000;
    expect(nextDeadline(state, now)).toBe(now + 2000);
    expect(nextDeadline(state, now + 2001)).toBeUndefined();
  });
  it('only displays explicitly pinned persisted tasks and retains their timing rules', () => {
    const state = initialSnapshot(now);
    state.storage = 'sqlite';
    state.todos[0].displayMode = 'NONE';
    state.todos[1].displayMode = 'EDGE';
    state.todos[1].snoozedUntil = now + 1000;
    state.todos[2].displayMode = 'EDGE';
    state.todos[2].completed = true;
    state.todos[2].completedAt = now;
    expect(visibleTodos(state, now).map(todo => todo.id)).not.toContain(1);
    expect(visibleTodos(state, now).map(todo => todo.id)).not.toContain(2);
    expect(visibleTodos(state, now + 799).map(todo => todo.id)).toContain(3);
    expect(visibleTodos(state, now + 800).map(todo => todo.id)).not.toContain(3);
    expect(visibleTodos(state, now + 1000).map(todo => todo.id)).toContain(2);
  });
  it('accepts reactive proxy snapshots without mutating their source', () => {
    const source = initialSnapshot(now);
    const reactive = new Proxy(source, {});
    const result = reduce(reactive, { type: 'complete', id: 1 }, now);
    expect(result.todos[0].completed).toBe(true);
    expect(source.todos[0].completed).toBe(false);
  });
});

it('changes visual urgency at 30 minutes and at the deadline without a recurring timer', () => {
  const now = 10000000;
  const todo = { ...initialSnapshot(now).todos[0], dueAt: now + 31 * 60000 };
  const state = { ...initialSnapshot(now), todos: [todo] };
  expect(dueState(todo, now)).toBe('none');
  expect(nextDeadline(state, now)).toBe(now + 60000);
  expect(dueState(todo, now + 60000)).toBe('soon');
  expect(nextDeadline(state, now + 60000)).toBe(todo.dueAt);
  expect(dueState(todo, todo.dueAt)).toBe('overdue');
  expect(nextDeadline(state, todo.dueAt)).toBeUndefined();
  expect(dueState({ ...todo, completed: true }, todo.dueAt)).toBe('none');
  expect(dueState({ ...todo, snoozedUntil: todo.dueAt + 1 }, todo.dueAt)).toBe('none');
});
