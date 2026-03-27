import { STORAGE_KEYS } from '~/utils/storageKeys';

export function useDisplayPrefs() {
  const timezone = useState('displayTimezone', () => {
    if (import.meta.client) {
      return localStorage.getItem(STORAGE_KEYS.timezone) || 'local';
    }
    return 'local';
  });

  const clockFormat = useState('displayClockFormat', () => {
    if (import.meta.client) {
      return localStorage.getItem(STORAGE_KEYS.clockFormat) || '12h';
    }
    return '12h';
  });

  function setTimezone(tz: string) {
    timezone.value = tz;
    if (import.meta.client) localStorage.setItem(STORAGE_KEYS.timezone, tz);
  }

  const viewMode = useState<'list' | 'grid'>('displayViewMode', () => {
    if (import.meta.client) {
      return (localStorage.getItem(STORAGE_KEYS.viewMode) as 'list' | 'grid') || 'list';
    }
    return 'list';
  });

  function setViewMode(mode: 'list' | 'grid') {
    viewMode.value = mode;
    if (import.meta.client) localStorage.setItem(STORAGE_KEYS.viewMode, mode);
  }

  const showExactDates = useState('displayExactDates', () => {
    if (import.meta.client) {
      return localStorage.getItem(STORAGE_KEYS.exactDates) === 'true';
    }
    return false;
  });

  function setClockFormat(fmt: string) {
    clockFormat.value = fmt;
    if (import.meta.client) localStorage.setItem(STORAGE_KEYS.clockFormat, fmt);
  }

  function setShowExactDates(val: boolean) {
    showExactDates.value = val;
    if (import.meta.client) localStorage.setItem(STORAGE_KEYS.exactDates, String(val));
  }

  function formatTimestamp(dateStr: string): string {
    const date = new Date(dateStr);
    const options: Intl.DateTimeFormatOptions = {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      hour12: clockFormat.value === '12h',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    };
    if (timezone.value !== 'local') {
      options.timeZone = timezone.value;
    }
    return new Intl.DateTimeFormat(undefined, options).format(date);
  }

  return {
    timezone,
    clockFormat,
    showExactDates,
    viewMode,
    setTimezone,
    setClockFormat,
    setShowExactDates,
    setViewMode,
    formatTimestamp,
  };
}
