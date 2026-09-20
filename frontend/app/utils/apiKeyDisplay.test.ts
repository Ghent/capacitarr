import { describe, it, expect } from 'vitest';
import { formatApiKeyForDisplay, isMaskedApiKey } from './apiKeyDisplay';

describe('apiKeyDisplay', () => {
  const fixtureKey = 'sonarr-test-api-key-firefly-serenity';

  it('does not expose a full plaintext fixture key in the display string', () => {
    const maskedFromApi = '••••••••••••••••••••••••••••nity';
    expect(isMaskedApiKey(maskedFromApi)).toBe(true);
    expect(formatApiKeyForDisplay(maskedFromApi)).not.toContain(fixtureKey);
    expect(maskedFromApi).not.toContain(fixtureKey);
  });

  it('truncates long keys without echoing the full secret', () => {
    const displayed = formatApiKeyForDisplay(fixtureKey);
    expect(displayed).not.toBe(fixtureKey);
    expect(displayed.includes('sonarr-t')).toBe(true);
    expect(displayed.includes('nity')).toBe(true);
    expect(displayed.length).toBeLessThan(fixtureKey.length);
  });
});
