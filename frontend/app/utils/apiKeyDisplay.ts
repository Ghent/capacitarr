/** True when the API returned a masked key (bullet prefix), not the live secret. */
export function isMaskedApiKey(apiKey: string): boolean {
  return apiKey.includes('•');
}

/**
 * Compact display for integration API keys. Masked keys from the API already
 * hide the secret; this only truncates long strings for the card UI.
 */
export function formatApiKeyForDisplay(apiKey: string): string {
  if (apiKey.length > 16) {
    return apiKey.slice(0, 8) + '••••' + apiKey.slice(-4);
  }
  return apiKey;
}
