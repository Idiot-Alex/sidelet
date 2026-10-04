// No references to components or DOM nodes are kept by these counters.
export const memoryEnabled = typeof location !== 'undefined' && new URLSearchParams(location.search).get('memory') === '1';
export function lifecycleCounters() {
  const counters: Record<string, { created: number; live: number }> = {};
  return {
    track(name: string) {
      const count = counters[name] ??= { created: 0, live: 0 };
      count.created++; count.live++;
      let disposed = false;
      return () => { if (!disposed) { count.live--; disposed = true; } };
    },
    snapshot: () => Object.fromEntries(Object.entries(counters).map(([name, count]) => [name, { ...count }])),
  };
}
const counts = lifecycleCounters();
export function trackMemory(name: string) { return memoryEnabled ? counts.track(name) : () => {}; }
export function memorySnapshot() {
  return { lifecycle: counts.snapshot(), domElements: document.querySelectorAll('*').length,
    hitElements: document.querySelectorAll('[data-hit]').length,
    quickCards: document.querySelectorAll('.quick-card').length,
    // Counts cover our registrations/components, not the JS engine's full heap.
    scope: 'Sidelet component and registration counters; current DOM only' };
}
