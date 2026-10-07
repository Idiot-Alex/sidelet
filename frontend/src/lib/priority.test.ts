import { expect, it } from 'vitest';
import { normalizePriority, priorityLabel } from './priority';

it('keeps saved normal, important and urgent levels unchanged', () => {
  for (const [value, label] of [[0, '普通'], [2, '重要'], [3, '紧急']] as const) {
    expect(normalizePriority(value)).toBe(value);
    expect(priorityLabel(value)).toBe(label);
  }
  expect(normalizePriority()).toBe(0);
});

it('shows and edits the retired higher level as important', () => {
  expect(priorityLabel(1)).toBe('重要');
  expect(normalizePriority(1)).toBe(2);
});
