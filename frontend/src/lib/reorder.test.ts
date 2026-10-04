import { describe, expect, it } from 'vitest';
import { dropTarget, moveTask } from './reorder';

describe('Task drag order', () => {
  const rows = [{ id: 1, top: 100, bottom: 144 }, { id: 2, top: 150, bottom: 194 }, { id: 3, top: 200, bottom: 244 }];
  it('uses row midpoints and the six-point transparent gaps', () => {
    expect(dropTarget(rows, 3, 110)).toEqual({ id: 1, after: false });
    expect(dropTarget(rows, 1, 197)).toEqual({ id: 3, after: false });
    expect(dropTarget(rows, 1, 243)).toEqual({ id: 3, after: true });
    expect(dropTarget(rows.slice(0, 1), 1, 130)).toBeUndefined();
  });
  it('anchors a drop to visible rows while retaining the overflow tail', () => {
    const ids = [1,2,3,4,5,6,7,8,9,10];
    expect(moveTask(ids, 1, { id: 3, after: true })).toEqual([2,3,1,4,5,6,7,8,9,10]);
    expect(moveTask(ids, 10, { id: 1, after: false })).toEqual([10,1,2,3,4,5,6,7,8,9]);
    expect(ids).toEqual([1,2,3,4,5,6,7,8,9,10]);
  });
  it('treats missing anchors and drops in the original slot as no-ops', () => {
    expect(moveTask([1,2,3], 2, { id: 3, after: false })).toEqual([1,2,3]);
    expect(moveTask([1,2,3], 2, { id: 99, after: false })).toEqual([1,2,3]);
    expect(moveTask([1,2,3], 99, { id: 1, after: true })).toEqual([1,2,3]);
  });
});
