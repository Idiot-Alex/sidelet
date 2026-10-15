export function quickCardHeight(height: number, scrollHeight: number, clientHeight: number, editing = false): number {
  if (editing) return 390;
  return Math.max(160, Math.min(390, Math.ceil(height + Math.max(0, scrollHeight - clientHeight))));
}

export function measureQuickCard(): number {
  const card = document.querySelector<HTMLElement>('.quick-card');
  const body = card?.querySelector<HTMLElement>('.quick-body');
  return card && body ? quickCardHeight(card.getBoundingClientRect().height, body.scrollHeight, body.clientHeight, card.classList.contains('editing')) : 390;
}
