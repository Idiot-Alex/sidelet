import { describe, expect, it } from 'vitest';
import { lifecycleCounters } from './memory';

describe('memory lifecycle counters', () => {
  it('balances repeated component destruction and tolerates duplicate cleanup', () => {
    const counts = lifecycleCounters();
    const permanent = counts.track('card');
    for (let i = 0; i < 100; i++) { const dispose = counts.track('card'); dispose(); dispose(); }
    expect(counts.snapshot().card).toEqual({ created: 101, live: 1 });
    permanent();
    expect(counts.snapshot().card.live).toBe(0);
  });
  it('snapshots cannot mutate subsequent observations', () => {
    const counts = lifecycleCounters(); counts.track('listener');
    counts.snapshot().listener.live = 99;
    expect(counts.snapshot().listener.live).toBe(1);
  });
});
