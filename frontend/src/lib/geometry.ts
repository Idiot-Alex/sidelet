export interface Rect { x: number; y: number; width: number; height: number }
export type Side = 'left' | 'right';

export function ensureVisible(rect: Rect, area: Rect, margin = 10): Rect {
  const marginX = Math.min(Math.max(0, margin), Math.max(0, area.width) / 2);
  const marginY = Math.min(Math.max(0, margin), Math.max(0, area.height) / 2);
  const width = Math.min(Math.max(0, rect.width), Math.max(0, area.width - 2 * marginX));
  const height = Math.min(Math.max(0, rect.height), Math.max(0, area.height - 2 * marginY));
  return {
    width, height,
    x: Math.max(area.x + marginX, Math.min(rect.x, area.x + area.width - marginX - width)),
    y: Math.max(area.y + marginY, Math.min(rect.y, area.y + area.height - marginY - height)),
  };
}

export function stackLayout(count: number, areaHeight: number, offset: number, itemHeight = 44, gap = 6, margin = 10) {
  margin = Math.min(Math.max(0, margin), Math.max(0, areaHeight) / 2);
  const capacity = Math.max(0, Math.floor((areaHeight - margin * 2 + gap) / (itemHeight + gap)));
  const direct = Math.min(count, 8, count > Math.min(capacity, 8) ? Math.max(0, capacity - 1) : capacity);
  const overflow = count - direct;
  const rows = direct + (overflow > 0 ? 1 : 0);
  const rowHeight = capacity === 0 ? Math.min(itemHeight, Math.max(0, areaHeight - 2 * margin)) : itemHeight;
  const height = Math.max(0, rows * rowHeight + Math.max(0, rows - 1) * gap);
  const top = ensureVisible({ x: 0, y: areaHeight * Math.max(0, Math.min(1, offset)) - height / 2, width: 0, height }, { x: 0, y: 0, width: 320, height: areaHeight }, margin).y;
  return { direct, overflow, height, top, capacity, rowHeight };
}

// Keep keyboard selection visible without changing the fixture's stored order.
export function stackItems<T extends { id: number }>(todos: T[], capacity: number, revealID = 0) {
  const direct = todos.slice(0, capacity);
  const reveal = todos.find(todo => todo.id === revealID);
  if (direct.length && reveal && !direct.some(todo => todo.id === revealID)) direct[direct.length - 1] = reveal;
  const ids = new Set(direct.map(todo => todo.id));
  return { direct, overflow: todos.filter(todo => !ids.has(todo.id)) };
}

export function quickRect(anchor: Rect, area: Rect, side: Side): Rect {
  const width = 320;
  return ensureVisible({ x: side === 'right' ? anchor.x - width - 12 : anchor.x + anchor.width + 12, y: anchor.y - 12, width, height: 390 }, area);
}

// Offset always refers to the whole WorkArea, even while the arrange controls
// reserve space. Thus entering/exiting arrange does not change its meaning.
export function arrangeStackLayout(count: number, areaHeight: number, offset: number, itemHeight = 44) {
  const layout = stackLayout(count, Math.max(0, areaHeight - 132), offset, itemHeight);
  const low = 52, high = Math.max(low, areaHeight - 100 - layout.height);
  return { ...layout, top: Math.max(low, Math.min(high, areaHeight * offset - layout.height / 2)) };
}
