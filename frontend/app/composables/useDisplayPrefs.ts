export function useDisplayPrefs() {
  const timezone = useState('displayTimezone', () => {
    if (import.meta.client) {
      return localStorage.getItem('capacitarr_timezone') || 'local'
    }
    return 'local'
  })

  const clockFormat = useState('displayClockFormat', () => {
    if (import.meta.client) {
      return localStorage.getItem('capacitarr_clockFormat') || '12h'
    }
    return '12h'
  })

  function setTimezone(tz: string) {
    timezone.value = tz
    if (import.meta.client) localStorage.setItem('capacitarr_timezone', tz)
  }

  function setClockFormat(fmt: string) {
    clockFormat.value = fmt
    if (import.meta.client) localStorage.setItem('capacitarr_clockFormat', fmt)
  }

  function formatTimestamp(dateStr: string): string {
    const date = new Date(dateStr)
    const options: Intl.DateTimeFormatOptions = {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      hour12: clockFormat.value === '12h',
      month: 'short',
      day: 'numeric',
      year: 'numeric'
    }
    if (timezone.value !== 'local') {
      options.timeZone = timezone.value
    }
    return new Intl.DateTimeFormat(undefined, options).format(date)
  }

  return { timezone, clockFormat, setTimezone, setClockFormat, formatTimestamp }
}
