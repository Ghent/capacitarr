import { describe, expect, it } from 'vitest';
import { EVENT_DELETION_FAILED, EVENT_ENGINE_COMPLETE, EVENT_ENGINE_START } from '~/constants';
import { eventIcon, eventIconClass } from './eventIcons';

describe('eventIcons', () => {
  it('maps known event types to distinct icons', () => {
    expect(eventIcon(EVENT_ENGINE_START)).not.toBe(eventIcon(EVENT_ENGINE_COMPLETE));
    expect(eventIcon(EVENT_DELETION_FAILED)).not.toBe(eventIcon(EVENT_ENGINE_COMPLETE));
  });

  it('returns a default icon for unknown types', () => {
    expect(eventIcon('not_a_real_event')).toBeTruthy();
  });

  it('uses success class for engine_complete', () => {
    expect(eventIconClass(EVENT_ENGINE_COMPLETE)).toBe('text-success');
  });

  it('uses destructive class for deletion_failed', () => {
    expect(eventIconClass(EVENT_DELETION_FAILED)).toBe('text-destructive');
  });
});
