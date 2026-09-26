import { describe, expect, it } from 'vitest';
import { MODE_APPROVAL, MODE_AUTO, MODE_DRY_RUN, MODE_SUNSET } from '~/constants';
import { modeLabelKey, mostAggressiveMode } from './diskGroupMode';

describe('mostAggressiveMode', () => {
  it('prefers auto over every other preset', () => {
    expect(mostAggressiveMode([MODE_DRY_RUN, MODE_SUNSET, MODE_APPROVAL, MODE_AUTO])).toBe(
      MODE_AUTO,
    );
  });

  it('prefers approval over sunset and dry-run', () => {
    expect(mostAggressiveMode([MODE_DRY_RUN, MODE_SUNSET, MODE_APPROVAL])).toBe(MODE_APPROVAL);
  });

  it('returns dry-run for an empty set', () => {
    expect(mostAggressiveMode([])).toBe(MODE_DRY_RUN);
  });
});

describe('modeLabelKey', () => {
  it('maps each preset to its i18n key', () => {
    expect(modeLabelKey(MODE_AUTO)).toBe('mode.auto');
    expect(modeLabelKey(MODE_APPROVAL)).toBe('mode.approval');
    expect(modeLabelKey(MODE_SUNSET)).toBe('mode.sunset');
    expect(modeLabelKey(MODE_DRY_RUN)).toBe('mode.dryRun');
  });
});
