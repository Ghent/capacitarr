/**
 * Client-side Plex OAuth flow.
 *
 * PIN creation and polling happen directly from the browser to plex.tv,
 * preserving the browser's device context so Plex can match an existing
 * session and skip re-authentication when the user is already logged in.
 *
 * Adapted from Maintainerr's PlexAuth.ts — rewritten without external
 * dependencies (no Bowser, no axios).
 */
import { STORAGE_KEYS } from '~/utils/storageKeys';

const PLEX_PIN_URL = 'https://plex.tv/api/v2/pins';
const PLEX_AUTH_URL = 'https://app.plex.tv/auth';
const CLIENT_ID_KEY = STORAGE_KEYS.plexClientId;
const POLL_INTERVAL_MS = 1_000;
const TIMEOUT_MS = 300_000; // 5 minutes
const POPUP_WIDTH = 600;
const POPUP_HEIGHT = 700;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface PlexHeaders extends Record<string, string> {
  Accept: string;
  'X-Plex-Product': string;
  'X-Plex-Version': string;
  'X-Plex-Client-Identifier': string;
  'X-Plex-Model': string;
  'X-Plex-Platform': string;
  'X-Plex-Platform-Version': string;
  'X-Plex-Device': string;
  'X-Plex-Device-Name': string;
  'X-Plex-Device-Screen-Resolution': string;
  'X-Plex-Language': string;
}

interface PlexPin {
  id: number;
  code: string;
}

interface UAInfo {
  platform: string;
  platformVersion: string;
  browserName: string;
  browserVersion: string;
}

// ---------------------------------------------------------------------------
// Lightweight UA detection
// ---------------------------------------------------------------------------

/**
 * Detect platform, browser name, and version using the modern
 * `navigator.userAgentData` API when available, falling back to manual
 * `navigator.userAgent` parsing.  No external libraries required.
 */
function detectUA(): UAInfo {
  // Try the modern API first (Chromium 90+)
  const uaData = (navigator as NavigatorWithUAData).userAgentData;
  if (uaData) {
    const platform = uaData.platform || detectPlatformFromUA();
    const brand = pickBrand(uaData.brands ?? []);
    return {
      platform,
      platformVersion: '', // high-entropy; unavailable synchronously
      browserName: brand.brand,
      browserVersion: brand.version,
    };
  }

  // Fallback: parse the UA string
  return parseUserAgent(navigator.userAgent);
}

/** Chromium UA-CH brand list type */
interface NavigatorUABrand {
  brand: string;
  version: string;
}

interface NavigatorUAData {
  platform: string;
  brands?: NavigatorUABrand[];
}

interface NavigatorWithUAData extends Navigator {
  userAgentData?: NavigatorUAData;
}

/** Pick the most specific brand from the Chromium brand list. */
function pickBrand(brands: NavigatorUABrand[]): { brand: string; version: string } {
  // Prefer well-known browser brands over the generic "Chromium" entry
  const preferred = ['Microsoft Edge', 'Opera', 'Brave', 'Vivaldi', 'Google Chrome', 'Chrome'];
  for (const name of preferred) {
    const match = brands.find((b) => b.brand === name);
    if (match) return { brand: match.brand, version: match.version };
  }
  // Fall back to whatever is available (skip the GREASE brand "Not A;Brand" etc.)
  const real = brands.find((b) => !b.brand.includes('Not'));
  if (real) return { brand: real.brand, version: real.version };
  return { brand: 'Unknown', version: '' };
}

/** Detect platform from the legacy UA string. */
function detectPlatformFromUA(): string {
  const ua = navigator.userAgent;
  if (/iPad|iPhone|iPod/.test(ua)) return 'iOS';
  if (/Android/.test(ua)) return 'Android';
  if (/Mac OS X/.test(ua)) return 'macOS';
  if (/Windows/.test(ua)) return 'Windows';
  if (/Linux/.test(ua)) return 'Linux';
  if (/CrOS/.test(ua)) return 'Chrome OS';
  return 'Unknown';
}

/** Full fallback parser for browsers without `userAgentData`. */
function parseUserAgent(ua: string): UAInfo {
  const platform = detectPlatformFromUA();

  // Extract platform version
  let platformVersion = '';
  const macMatch = ua.match(/Mac OS X ([\d._]+)/);
  if (macMatch) platformVersion = macMatch[1]!.replace(/_/g, '.');
  const winMatch = ua.match(/Windows NT ([\d.]+)/);
  if (winMatch) platformVersion = winMatch[1]!;
  const androidMatch = ua.match(/Android ([\d.]+)/);
  if (androidMatch) platformVersion = androidMatch[1]!;
  const iosMatch = ua.match(/OS ([\d_]+) like Mac OS X/);
  if (iosMatch) platformVersion = iosMatch[1]!.replace(/_/g, '.');

  // Detect browser — order matters (more specific first)
  let browserName = 'Unknown';
  let browserVersion = '';

  const browserPatterns: [string, RegExp][] = [
    ['Edge', /Edg(?:e|A|iOS)?\/([\d.]+)/],
    ['Opera', /OPR\/([\d.]+)/],
    ['Vivaldi', /Vivaldi\/([\d.]+)/],
    ['Brave', /Brave\/([\d.]+)/],
    ['Samsung Internet', /SamsungBrowser\/([\d.]+)/],
    ['Firefox', /Firefox\/([\d.]+)/],
    ['Safari', /Version\/([\d.]+).*Safari/],
    ['Chrome', /Chrome\/([\d.]+)/],
  ];

  for (const [name, regex] of browserPatterns) {
    const match = ua.match(regex);
    if (match) {
      browserName = name;
      browserVersion = match[1]!;
      break;
    }
  }

  return { platform, platformVersion, browserName, browserVersion };
}

// ---------------------------------------------------------------------------
// Client ID persistence
// ---------------------------------------------------------------------------

/**
 * Generate a UUID v4 using `crypto.getRandomValues`.
 *
 * Unlike `crypto.randomUUID()`, `getRandomValues` is available in
 * **non-secure contexts** (plain HTTP on a LAN IP), which is common
 * for self-hosted apps.
 */
function generateUUID(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  // Set version (4) and variant (RFC 4122)
  bytes[6] = (bytes[6]! & 0x0f) | 0x40;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

/** Return a persistent per-browser UUID, generating one if absent. */
function getClientId(): string {
  let id = localStorage.getItem(CLIENT_ID_KEY);
  if (!id) {
    id = generateUUID();
    localStorage.setItem(CLIENT_ID_KEY, id);
  }
  return id;
}

// ---------------------------------------------------------------------------
// PlexOAuth class
// ---------------------------------------------------------------------------

export class PlexOAuth {
  private headers: PlexHeaders;
  private pin: PlexPin | null = null;
  private popup: Window | null = null;
  private pollTimer: ReturnType<typeof setTimeout> | null = null;
  private timeoutTimer: ReturnType<typeof setTimeout> | null = null;
  private aborted = false;

  constructor() {
    const ua = detectUA();
    const clientId = getClientId();

    this.headers = {
      Accept: 'application/json',
      'X-Plex-Product': 'Capacitarr',
      'X-Plex-Version': '1.0.0',
      'X-Plex-Client-Identifier': clientId,
      'X-Plex-Model': 'Plex OAuth',
      'X-Plex-Platform': ua.platform,
      'X-Plex-Platform-Version': ua.platformVersion || 'Unknown',
      'X-Plex-Device': ua.browserName,
      'X-Plex-Device-Name': `${ua.browserName} ${ua.browserVersion}`.trim() || 'Unknown',
      'X-Plex-Device-Screen-Resolution': `${window.screen.width}x${window.screen.height}`,
      'X-Plex-Language': navigator.language,
    };
  }

  // -------------------------------------------------------------------------
  // Public API
  // -------------------------------------------------------------------------

  /**
   * Full OAuth flow:
   * 1. Create PIN (browser → plex.tv)
   * 2. Open popup to Plex auth page
   * 3. Poll for token every 1 s
   * 4. Resolve with `authToken` or reject on error / timeout / user close
   */
  async login(): Promise<string> {
    this.aborted = false;

    // Step 1 — create PIN
    await this.createPin();

    // Step 2 — open popup (about:blank first, then redirect)
    this.openPopup();
    if (!this.popup) {
      throw new Error('Popup blocked by browser. Please allow popups for this site.');
    }
    this.popup.location.href = this.buildAuthUrl();

    // Step 3 — poll for token
    return this.pollForToken();
  }

  /** Cancel an in-progress login flow and close the popup. */
  abort(): void {
    this.aborted = true;
    this.cleanup();
  }

  // -------------------------------------------------------------------------
  // PIN creation
  // -------------------------------------------------------------------------

  private async createPin(): Promise<void> {
    const response = await fetch(`${PLEX_PIN_URL}?strong=true`, {
      method: 'POST',
      headers: this.headers,
    });

    if (!response.ok) {
      throw new Error(`Failed to create Plex PIN: ${response.status} ${response.statusText}`);
    }

    const data = await response.json();
    this.pin = { id: data.id, code: data.code };
  }

  // -------------------------------------------------------------------------
  // Popup management
  // -------------------------------------------------------------------------

  private openPopup(): void {
    // Dual-screen–safe centering
    const screenLeft = window.screenLeft ?? window.screenX;
    const screenTop = window.screenTop ?? window.screenY;
    const width = window.innerWidth || document.documentElement.clientWidth || screen.width;
    const height = window.innerHeight || document.documentElement.clientHeight || screen.height;
    const left = width / 2 - POPUP_WIDTH / 2 + screenLeft;
    const top = height / 2 - POPUP_HEIGHT / 2 + screenTop;

    // Open about:blank first to avoid popup-blocker heuristics.
    // Browsers allow window.open() inside user-gesture handlers when the URL
    // is innocuous; the real auth URL is set immediately after.
    const win = window.open(
      'about:blank',
      'PlexOAuth',
      `scrollbars=yes,width=${POPUP_WIDTH},height=${POPUP_HEIGHT},top=${top},left=${left}`,
    );

    if (win) {
      win.focus();
      this.popup = win;
    }
  }

  private closePopup(): void {
    try {
      this.popup?.close();
    } catch (err) {
      console.warn('[PlexOAuth] closePopup failed:', err);
    }
    this.popup = null;
  }

  // -------------------------------------------------------------------------
  // Auth URL
  // -------------------------------------------------------------------------

  private buildAuthUrl(): string {
    if (!this.pin) throw new Error('PIN not initialised');

    const params: Record<string, string> = {
      clientID: this.headers['X-Plex-Client-Identifier'],
      code: this.pin.code,
      'context[device][product]': this.headers['X-Plex-Product'],
      'context[device][version]': this.headers['X-Plex-Version'],
      'context[device][platform]': this.headers['X-Plex-Platform'],
      'context[device][platformVersion]': this.headers['X-Plex-Platform-Version'],
      'context[device][device]': this.headers['X-Plex-Device'],
      'context[device][deviceName]': this.headers['X-Plex-Device-Name'],
      'context[device][model]': this.headers['X-Plex-Model'],
      'context[device][screenResolution]': this.headers['X-Plex-Device-Screen-Resolution'],
      'context[device][layout]': 'desktop',
      'context[device][environment]': 'bundled',
    };

    const qs = Object.entries(params)
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
      .join('&');

    // Note the `#!` — the exclamation mark forces Plex's SPA router to
    // handle the auth route correctly.
    return `${PLEX_AUTH_URL}/#!?${qs}`;
  }

  // -------------------------------------------------------------------------
  // Polling
  // -------------------------------------------------------------------------

  private pollForToken(): Promise<string> {
    return new Promise<string>((resolve, reject) => {
      // 5-minute hard timeout
      this.timeoutTimer = setTimeout(() => {
        this.cleanup();
        reject(new Error('Plex login timed out after 5 minutes'));
      }, TIMEOUT_MS);

      const poll = async () => {
        if (this.aborted) {
          reject(new Error('Plex login was cancelled'));
          return;
        }

        try {
          if (!this.pin) {
            throw new Error('PIN not initialised');
          }

          const response = await fetch(`${PLEX_PIN_URL}/${this.pin.id}`, {
            headers: this.headers,
          });

          if (!response.ok) {
            throw new Error(`PIN poll failed: ${response.status} ${response.statusText}`);
          }

          const data = await response.json();

          if (data.authToken) {
            this.cleanup();
            resolve(data.authToken as string);
          } else if (!this.popup?.closed) {
            // No token yet, popup still open — keep polling
            this.pollTimer = setTimeout(poll, POLL_INTERVAL_MS);
          } else {
            // Popup was closed without completing login
            this.cleanup();
            reject(new Error('User closed the popup without completing Plex login'));
          }
        } catch (err) {
          this.cleanup();
          reject(err);
        }
      };

      // Kick off the first poll after one interval
      this.pollTimer = setTimeout(poll, POLL_INTERVAL_MS);
    });
  }

  // -------------------------------------------------------------------------
  // Cleanup
  // -------------------------------------------------------------------------

  private cleanup(): void {
    if (this.pollTimer !== null) {
      clearTimeout(this.pollTimer);
      this.pollTimer = null;
    }
    if (this.timeoutTimer !== null) {
      clearTimeout(this.timeoutTimer);
      this.timeoutTimer = null;
    }
    this.closePopup();
    this.pin = null;
  }
}
