import { describe, it, expect } from 'vitest';
import { factorColor, factorAbbr } from './factorColors';

// ---------------------------------------------------------------------------
// factorColor
// ---------------------------------------------------------------------------
describe('factorColor', () => {
  it.each([
    ['watch_history', '#8b5cf6'],
    ['last_watched', '#3b82f6'],
    ['file_size', '#f59e0b'],
    ['rating', '#10b981'],
    ['time_in_library', '#f97316'],
    ['series_status', '#ec4899'],
    ['request_popularity', '#06b6d4'],
  ])('returns correct color for key %s', (key, expected) => {
    expect(factorColor(key)).toBe(expected);
  });

  it('returns default gray for unknown key', () => {
    expect(factorColor('unknown')).toBe('#6b7280');
  });

  it('returns default gray when key is undefined', () => {
    expect(factorColor(undefined)).toBe('#6b7280');
  });

  it('returns default gray when key is empty string', () => {
    expect(factorColor('')).toBe('#6b7280');
  });
});

// ---------------------------------------------------------------------------
// factorAbbr
// ---------------------------------------------------------------------------
describe('factorAbbr', () => {
  it.each([
    ['watch_history', 'P:'],
    ['last_watched', 'LP:'],
    ['file_size', 'S:'],
    ['rating', 'Rt:'],
    ['time_in_library', 'A:'],
    ['series_status', 'Sh:'],
    ['request_popularity', 'Rq:'],
  ])('returns correct abbreviation for key %s', (key, expected) => {
    expect(factorAbbr(key)).toBe(expected);
  });

  it('generates abbreviation from first two chars for unknown key', () => {
    expect(factorAbbr('custom_factor')).toBe('cu:');
  });

  it('returns empty abbreviation when key is undefined', () => {
    expect(factorAbbr(undefined)).toBe(':');
  });
});
