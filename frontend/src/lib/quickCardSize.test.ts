import { expect, it } from 'vitest';
import { quickCardHeight } from './quickCardSize';

it('recovers content height when the previous popup viewport was shorter', () => {
  expect(quickCardHeight(180, 250, 80)).toBe(350);
  expect(quickCardHeight(350, 250, 250)).toBe(350);
});

it('keeps short cards compact, long notes bounded and editing spacious', () => {
  expect(quickCardHeight(120, 24, 24)).toBe(160);
  expect(quickCardHeight(390, 4000, 240)).toBe(390);
  expect(quickCardHeight(180, 40, 40, true)).toBe(390);
});
