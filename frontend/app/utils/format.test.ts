import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  formatBytes,
  formatPercent,
  formatTime,
  diskUsageStatus,
  diskStatusBgClass,
  diskStatusTextClass,
  diskStatusFillColor,
  formatRelativeTime,
} from './format'

// ---------------------------------------------------------------------------
// formatBytes
// ---------------------------------------------------------------------------
describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [1, '1.0 B'],
    [100, '100 B'],
    [512, '512 B'],
    [1023, '1023 B'],
  ])('handles small values: formatBytes(%i) → %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })

  it.each([
    [1024, '1.0 KB'],
    [1536, '1.5 KB'],
    [10240, '10.0 KB'],
    [102400, '100 KB'],
    [524288, '512 KB'],
  ])('formats kilobytes: formatBytes(%i) → %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })

  it.each([
    [1048576, '1.0 MB'],
    [1572864, '1.5 MB'],
    [104857600, '100 MB'],
    [536870912, '512 MB'],
  ])('formats megabytes: formatBytes(%i) → %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })

  it.each([
    [1073741824, '1.0 GB'],
    [5368709120, '5.0 GB'],
    [107374182400, '100 GB'],
  ])('formats gigabytes: formatBytes(%i) → %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })

  it.each([
    [1099511627776, '1.0 TB'],
    [5497558138880, '5.0 TB'],
  ])('formats terabytes: formatBytes(%i) → %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })

  it('handles negative values using absolute value for unit selection', () => {
    const result = formatBytes(-1073741824)
    // Negative input should produce a formatted string with negative sign
    expect(result).toBe('-1.0 GB')
  })

  it('returns "0 B" for NaN-like falsy values', () => {
    // The function checks `!bytes` first, which catches NaN and undefined-like coerced values
    expect(formatBytes(NaN)).toBe('0 B')
  })

  it('applies precision correctly — no decimal for values >= 100', () => {
    // 100 KB = 102400 bytes
    const result = formatBytes(102400)
    expect(result).toBe('100 KB')
    // No .0 suffix because val >= 100 triggers toFixed(0)
    expect(result).not.toContain('.')
  })

  it('applies precision correctly — one decimal for values < 100', () => {
    // 99.5 KB ≈ 101888 bytes
    const result = formatBytes(101888)
    expect(result).toMatch(/^\d+\.\d KB$/)
  })
})

// ---------------------------------------------------------------------------
// formatPercent
// ---------------------------------------------------------------------------
describe('formatPercent', () => {
  it('formats with default 0 decimals', () => {
    expect(formatPercent(75)).toBe('75%')
    expect(formatPercent(99.7)).toBe('100%')
  })

  it('formats with specified decimals', () => {
    expect(formatPercent(75.123, 1)).toBe('75.1%')
    expect(formatPercent(75.123, 2)).toBe('75.12%')
  })

  it('handles 0 and 100', () => {
    expect(formatPercent(0)).toBe('0%')
    expect(formatPercent(100)).toBe('100%')
  })
})

// ---------------------------------------------------------------------------
// formatTime
// ---------------------------------------------------------------------------
describe('formatTime', () => {
  it('returns "Never" for null', () => {
    expect(formatTime(null)).toBe('Never')
  })

  it('returns "Never" for undefined', () => {
    expect(formatTime(undefined)).toBe('Never')
  })

  it('returns "Never" for empty string', () => {
    expect(formatTime('')).toBe('Never')
  })

  it('formats a valid ISO date string', () => {
    const result = formatTime('2025-01-15T10:30:00Z')
    // toLocaleString output varies by environment, just verify it's not "Never"
    expect(result).not.toBe('Never')
    expect(result.length).toBeGreaterThan(0)
  })
})

// ---------------------------------------------------------------------------
// diskUsageStatus
// ---------------------------------------------------------------------------
describe('diskUsageStatus', () => {
  // Standard case: target=70, threshold=85
  it.each([
    [50, 70, 85, 'ok'],
    [69.9, 70, 85, 'ok'],
    [70, 70, 85, 'warning'],
    [80, 70, 85, 'warning'],
    [84.9, 70, 85, 'warning'],
    [85, 70, 85, 'danger'],
    [95, 70, 85, 'danger'],
    [100, 70, 85, 'danger'],
    [0, 70, 85, 'ok'],
  ] as const)('diskUsageStatus(%f, %f, %f) → %s', (usage, target, threshold, expected) => {
    expect(diskUsageStatus(usage, target, threshold)).toBe(expected)
  })

  it('handles edge case where target equals threshold', () => {
    // usage < target → ok, usage >= threshold (=target) → danger
    expect(diskUsageStatus(49, 50, 50)).toBe('ok')
    expect(diskUsageStatus(50, 50, 50)).toBe('danger') // threshold check comes first
  })
})

// ---------------------------------------------------------------------------
// diskStatusBgClass
// ---------------------------------------------------------------------------
describe('diskStatusBgClass', () => {
  it('returns green for ok status', () => {
    expect(diskStatusBgClass(30, 70, 85)).toBe('bg-green-500')
  })

  it('returns amber for warning status', () => {
    expect(diskStatusBgClass(75, 70, 85)).toBe('bg-amber-500')
  })

  it('returns red for danger status', () => {
    expect(diskStatusBgClass(90, 70, 85)).toBe('bg-red-500')
  })
})

// ---------------------------------------------------------------------------
// diskStatusTextClass
// ---------------------------------------------------------------------------
describe('diskStatusTextClass', () => {
  it('returns green text for ok status', () => {
    expect(diskStatusTextClass(30, 70, 85)).toBe('text-green-500')
  })

  it('returns amber text for warning status', () => {
    expect(diskStatusTextClass(75, 70, 85)).toBe('text-amber-500')
  })

  it('returns red text for danger status', () => {
    expect(diskStatusTextClass(90, 70, 85)).toBe('text-red-500')
  })
})

// ---------------------------------------------------------------------------
// diskStatusFillColor
// ---------------------------------------------------------------------------
describe('diskStatusFillColor', () => {
  it('returns green hex for ok status', () => {
    expect(diskStatusFillColor(30, 70, 85)).toBe('#22c55e')
  })

  it('returns yellow hex for warning status', () => {
    expect(diskStatusFillColor(75, 70, 85)).toBe('#eab308')
  })

  it('returns red hex for danger status', () => {
    expect(diskStatusFillColor(90, 70, 85)).toBe('#ef4444')
  })
})

// ---------------------------------------------------------------------------
// formatRelativeTime
// ---------------------------------------------------------------------------
describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2025-06-15T12:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns "Never" for null', () => {
    expect(formatRelativeTime(null)).toBe('Never')
  })

  it('returns "Never" for undefined', () => {
    expect(formatRelativeTime(undefined)).toBe('Never')
  })

  it('returns "Never" for empty string', () => {
    expect(formatRelativeTime('')).toBe('Never')
  })

  it('returns "just now" for times less than 60 seconds ago', () => {
    expect(formatRelativeTime('2025-06-15T11:59:30Z')).toBe('just now')
    expect(formatRelativeTime('2025-06-15T11:59:55Z')).toBe('just now')
  })

  it('returns minutes ago for 1-59 minutes', () => {
    expect(formatRelativeTime('2025-06-15T11:59:00Z')).toBe('1m ago')
    expect(formatRelativeTime('2025-06-15T11:30:00Z')).toBe('30m ago')
    expect(formatRelativeTime('2025-06-15T11:01:00Z')).toBe('59m ago')
  })

  it('returns hours ago for 1-23 hours', () => {
    expect(formatRelativeTime('2025-06-15T11:00:00Z')).toBe('1h ago')
    expect(formatRelativeTime('2025-06-15T00:00:00Z')).toBe('12h ago')
    expect(formatRelativeTime('2025-06-14T13:00:00Z')).toBe('23h ago')
  })

  it('returns days ago for 1-6 days', () => {
    expect(formatRelativeTime('2025-06-14T12:00:00Z')).toBe('1d ago')
    expect(formatRelativeTime('2025-06-12T12:00:00Z')).toBe('3d ago')
    expect(formatRelativeTime('2025-06-09T12:00:00Z')).toBe('6d ago')
  })

  it('returns locale date string for 7+ days ago', () => {
    const result = formatRelativeTime('2025-06-01T12:00:00Z')
    // 14 days ago — should fall through to toLocaleDateString
    expect(result).not.toBe('Never')
    expect(result).not.toContain('ago')
    expect(result).not.toBe('just now')
  })
})
