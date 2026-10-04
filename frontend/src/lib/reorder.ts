export interface DropTarget { id: number; after: boolean }
export interface RowBounds { id: number; top: number; bottom: number }

// Keep the drop anchored to a displayed row, including when +N hides a tail.
export function dropTarget(rows: RowBounds[], source: number, y: number): DropTarget | undefined {
  const targets = rows.filter(row => row.id !== source);
  for (const row of targets) {
    if (y < (row.top + row.bottom) / 2) return { id: row.id, after: false };
  }
  const last = targets.at(-1);
  return last ? { id: last.id, after: true } : undefined;
}

export function moveTask(ids: number[], source: number, target: DropTarget): number[] {
  if (source === target.id || !ids.includes(source) || !ids.includes(target.id)) return [...ids];
  const next = ids.filter(id => id !== source);
  next.splice(next.indexOf(target.id) + (target.after ? 1 : 0), 0, source);
  return next;
}
