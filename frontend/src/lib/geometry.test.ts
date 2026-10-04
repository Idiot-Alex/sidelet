import { describe, expect, it } from 'vitest';
import { arrangeStackLayout, ensureVisible, quickRect, stackItems, stackLayout } from './geometry';

describe('WorkArea geometry', () => {
  it('reserves a row for overflow when eight fixtures cannot fit', () => {
    const layout = stackLayout(8, 240, .9);
    expect(layout.direct).toBe(3);
    expect(layout.overflow).toBe(5);
    expect(layout.top + layout.height).toBeLessThanOrEqual(230);
  });
  it('uses the center preference while clamping the entire stack', () => {
    for (const height of [180, 240, 480, 800]) {
      for (const offset of [0, .35, 1]) {
        const layout = stackLayout(8, height, offset, 64);
        expect(layout.top).toBeGreaterThanOrEqual(10);
        expect(layout.top + layout.height).toBeLessThanOrEqual(height - 10);
        expect(layout.direct + layout.overflow).toBe(8);
        expect(layout.direct).toBeLessThanOrEqual(8);
      }
    }
  });
  it('limits a large card before clamping, including a negative-origin display', () => {
    const area = { x: -1920, y: -200, width: 1280, height: 300 };
    const rect = ensureVisible({ x: -2500, y: 500, width: 5000, height: 900 }, area);
    expect(rect).toEqual({ x: -1910, y: -190, width: 1260, height: 280 });
  });
  it('keeps either edge popup inside a short work area', () => {
    for (const side of ['left', 'right'] as const) {
      const rect = quickRect({ x: 680, y: 220, width: 14, height: 44 }, { x: 0, y: 0, width: 700, height: 240 }, side);
      expect(rect.x).toBeGreaterThanOrEqual(10);
      expect(rect.y).toBe(10);
      expect(rect.height).toBe(220);
      expect(rect.x + rect.width).toBeLessThanOrEqual(690);
    }
  });
  it('keeps an overflow-only entry inside a work area shorter than one label', () => {
    for (const height of [0, 8, 20, 40, 60]) {
      const layout = stackLayout(8, height, 1, 64);
      expect(layout.direct).toBe(0);
      expect(layout.overflow).toBe(8);
      expect(layout.top + layout.height).toBeLessThanOrEqual(height);
      expect(layout.rowHeight).toBeGreaterThanOrEqual(0);
      expect(ensureVisible({ x: 50, y: 50, width: 320, height: 390 }, { x: 0, y: 0, width: height, height })).toEqual(expect.objectContaining({ x: Math.min(10, height / 2), y: Math.min(10, height / 2) }));
    }
  });
  it('reveals a keyboard-selected overflow task while preserving order and unique ownership', () => {
    const todos = Array.from({ length: 8 }, (_, i) => ({ id: i + 1 }));
    const items = stackItems(todos, 2, 8);
    expect(items.direct.map(todo => todo.id)).toEqual([1, 8]);
    expect(items.overflow.map(todo => todo.id)).toEqual([2, 3, 4, 5, 6, 7]);
    expect(new Set([...items.direct, ...items.overflow].map(todo => todo.id)).size).toBe(8);
    expect(todos.map(todo => todo.id)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
    expect(stackItems(todos, 0, 8).overflow).toEqual(todos);
  });
});

it('keeps the same WorkArea center while reserving arrange controls', () => {
  const layout = arrangeStackLayout(4, 1000, .4);
  expect(layout.top + layout.height / 2).toBe(400);
  expect(arrangeStackLayout(4, 1000, 0).top).toBe(52);
  const bottom = arrangeStackLayout(30, 480, 1);
  expect(bottom.top + bottom.height).toBeLessThanOrEqual(380);
  expect(bottom.overflow).toBeGreaterThan(0);
});
