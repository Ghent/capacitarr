import { describe, expect, it } from 'vitest';
import { bucketHourly, prepareSeriesData } from './useEngineHistory';

describe('prepareSeriesData', () => {
  const points = [
    { timestamp: '2026-01-01T00:00:00Z', candidates: 2 },
    { timestamp: '2026-01-01T00:30:00Z', candidates: 3 },
  ];

  it('keeps raw points for short ranges', () => {
    const series = prepareSeriesData(points, 'candidates', '24h');
    expect(series).toHaveLength(2);
    expect(series[1]?.y).toBe(3);
  });

  it('buckets hourly for long ranges', () => {
    const series = bucketHourly(points, 'candidates');
    expect(series).toHaveLength(1);
    expect(series[0]?.y).toBe(5);
  });
});
