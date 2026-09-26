/**
 * Engine sparkline history — shared so dashboard and replay_gap can refetch.
 * Chart option builders live here. The card is a view; it does not register SSE.
 */
import type { DeletionProgress } from '~/types/api';

export interface EngineHistoryPoint {
  timestamp: string;
  evaluated: number;
  candidates: number;
  queued: number;
  deleted: number;
  freedBytes: number;
  durationMs: number;
  diskGroupModes: string;
}

export const DATE_RANGE_VALUES = ['1h', '6h', '24h', '7d', '30d', 'all'] as const;
export type DateRangeValue = (typeof DATE_RANGE_VALUES)[number];

const DATE_RANGE_KEYS: Record<DateRangeValue, string> = {
  '1h': 'dashboard.lastHour',
  '6h': 'dashboard.last6h',
  '24h': 'dashboard.last24h',
  '7d': 'dashboard.last7d',
  '30d': 'dashboard.last30d',
  all: 'dashboard.allTime',
};

export function bucketHourly(
  data: Array<{ timestamp: string }>,
  valueKey: string,
): Array<{ x: number; y: number }> {
  const buckets = new Map<number, { ts: number; sum: number }>();
  for (const point of data) {
    const ts = new Date(point.timestamp).getTime();
    const hourKey = Math.floor(ts / 3_600_000);
    const existing = buckets.get(hourKey);
    const value = (point as Record<string, unknown>)[valueKey] as number;
    if (existing) {
      existing.sum += value;
    } else {
      buckets.set(hourKey, { ts: hourKey * 3_600_000 + 1_800_000, sum: value });
    }
  }
  return Array.from(buckets.values())
    .sort((a, b) => a.ts - b.ts)
    .map((b) => ({ x: b.ts, y: b.sum }));
}

export function prepareSeriesData(
  data: Array<{ timestamp: string }>,
  valueKey: string,
  range: string,
): Array<{ x: number; y: number }> {
  if (range === '1h' || range === '6h' || range === '24h' || data.length <= 24) {
    return data.map((point) => ({
      x: new Date(point.timestamp).getTime(),
      y: (point as Record<string, unknown>)[valueKey] as number,
    }));
  }
  return bucketHourly(data, valueKey);
}

export function useEngineHistory() {
  const api = useApi();
  const { t } = useI18n();
  const {
    chart1Color,
    chart3Color,
    destructiveColor,
    successColor,
    glowLineStyle,
    gradientArea,
    tooltipConfig,
    emphasisConfig,
  } = useEChartsDefaults();
  const { isRunning: engineIsRunning } = useEngineControl();

  const dateRange = useState<DateRangeValue>('engineHistoryRange', () => '24h');
  const history = useState<EngineHistoryPoint[]>('engineHistoryData', () => []);

  const dateRangeOptions = computed(() =>
    DATE_RANGE_VALUES.map((value) => ({
      value,
      label: t(DATE_RANGE_KEYS[value]),
    })),
  );

  const dateRangeLabel = computed(() => {
    const match = dateRangeOptions.value.find((o) => o.value === dateRange.value);
    return match?.label ?? dateRange.value;
  });

  const candidatesSeries = computed(() =>
    prepareSeriesData(history.value, 'candidates', dateRange.value),
  );
  const queuedSeries = computed(() => prepareSeriesData(history.value, 'queued', dateRange.value));
  const deletedSeries = computed(() =>
    prepareSeriesData(history.value, 'deleted', dateRange.value),
  );
  const hasQueuedData = computed(() => queuedSeries.value.some((d) => d.y > 0));
  const hasDeletedData = computed(() => deletedSeries.value.some((d) => d.y > 0));

  const avgDurationMs = computed(() => {
    if (history.value.length === 0) return 0;
    const sum = history.value.reduce((acc, p) => acc + p.durationMs, 0);
    return Math.round(sum / history.value.length);
  });

  const maxDurationMs = computed(() => {
    if (history.value.length === 0) return 0;
    return Math.max(...history.value.map((p) => p.durationMs));
  });

  const sparklineEChartsOption = computed(() => {
    const candidates = candidatesSeries.value;
    const qData = queuedSeries.value;
    const dData = deletedSeries.value;
    const series: Array<Record<string, unknown>> = [];
    const sparseSymbol = (len: number) => (len <= 3 ? 'circle' : 'none');
    const sparseSymbolSize = (len: number) => (len <= 3 ? 6 : 0);

    if (candidates.length > 0) {
      series.push({
        name: t('dashboard.candidates'),
        type: 'line',
        smooth: true,
        symbol: sparseSymbol(candidates.length),
        symbolSize: sparseSymbolSize(candidates.length),
        itemStyle: { color: chart1Color.value },
        lineStyle: glowLineStyle(chart1Color.value),
        areaStyle: gradientArea(chart1Color.value),
        emphasis: emphasisConfig(),
        data: candidates.map((d) => [d.x, d.y]),
      });
    }
    if (qData.length > 0) {
      series.push({
        name: t('dashboard.wouldDelete'),
        type: 'line',
        smooth: true,
        symbol: sparseSymbol(qData.length),
        symbolSize: sparseSymbolSize(qData.length),
        itemStyle: { color: chart3Color.value },
        lineStyle: glowLineStyle(chart3Color.value),
        areaStyle: gradientArea(chart3Color.value),
        emphasis: emphasisConfig(),
        data: qData.map((d) => [d.x, d.y]),
      });
    }
    if (dData.length > 0) {
      const lastPoint = dData[dData.length - 1];
      series.push({
        name: t('dashboard.deleted'),
        type: 'line',
        smooth: true,
        symbol: sparseSymbol(dData.length),
        symbolSize: sparseSymbolSize(dData.length),
        itemStyle: { color: destructiveColor.value },
        lineStyle: glowLineStyle(destructiveColor.value),
        areaStyle: gradientArea(destructiveColor.value),
        emphasis: emphasisConfig(),
        data: dData.map((d) => [d.x, d.y]),
        markPoint:
          engineIsRunning.value && lastPoint
            ? {
                symbol: 'circle',
                symbolSize: 8,
                data: [{ coord: [lastPoint.x, lastPoint.y] }],
                itemStyle: { color: destructiveColor.value },
                animation: true,
                animationDuration: 1200,
                animationEasingUpdate: 'sinusoidalInOut',
              }
            : undefined,
      });
    }

    return {
      animation: true,
      animationDelay: (idx: number) => idx * 10,
      grid: { top: 4, right: 4, bottom: 4, left: 4 },
      xAxis: {
        type: 'time',
        show: false,
        axisPointer: {
          label: {
            formatter: (p: { value: number }) => new Date(p.value).toLocaleString(),
          },
        },
      },
      yAxis: {
        type: 'value',
        show: false,
        minInterval: 1,
        axisPointer: {
          label: { formatter: (p: { value: number }) => Math.round(p.value).toString() },
        },
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          crossStyle: { color: chart1Color.value, opacity: 0.3 },
        },
        ...tooltipConfig(),
        formatter: (
          params: Array<{ seriesName: string; value: [number, number]; marker: string }>,
        ) => {
          if (!params.length) return '';
          const ts = new Date(params[0]!.value[0]).toLocaleString();
          let html = `<div style="font-weight:600">${ts}</div>`;
          let pointHasQueued = false;
          let pointHasDeleted = false;
          for (const p of params) {
            if (p.value[1] === 0) continue;
            html += `<div>${p.marker} ${p.seriesName}: <b>${Math.round(p.value[1])}</b></div>`;
            if (p.seriesName === t('dashboard.wouldDelete')) pointHasQueued = true;
            if (p.seriesName === t('dashboard.deleted')) pointHasDeleted = true;
          }
          if (pointHasQueued && pointHasDeleted) {
            html += `<div style="opacity:0.6;font-size:11px;margin-top:2px">${t('dashboard.mixedModesHint')}</div>`;
          } else if (pointHasQueued) {
            html += `<div style="opacity:0.6;font-size:11px;margin-top:2px">${t('dashboard.dryRunNoDelete')}</div>`;
          }
          return html;
        },
      },
      series,
    };
  });

  const durationSparklineEChartsOption = computed(() => ({
    animation: true,
    grid: { top: 4, right: 4, bottom: 4, left: 4 },
    xAxis: { type: 'time' as const, show: false },
    yAxis: { type: 'value' as const, show: false },
    tooltip: {
      trigger: 'axis' as const,
      ...tooltipConfig(),
      formatter: (params: Array<{ value: [number, number] }>) => {
        if (!params[0]) return '';
        const [ts, val] = params[0].value;
        const date = new Date(ts).toLocaleString();
        return `${date}<br/>${val}ms`;
      },
    },
    visualMap: [
      {
        show: false,
        min: 0,
        max: maxDurationMs.value || 1,
        inRange: { color: [successColor.value, chart3Color.value, destructiveColor.value] },
      },
    ],
    series: [
      {
        name: 'Duration',
        type: 'line',
        smooth: true,
        symbol: 'none',
        lineStyle: glowLineStyle(successColor.value),
        areaStyle: gradientArea(successColor.value),
        emphasis: emphasisConfig(),
        data: history.value.map((p) => [new Date(p.timestamp).getTime(), p.durationMs]),
      },
    ],
  }));

  async function fetchHistory() {
    try {
      const range = dateRange.value || '7d';
      const data = (await api(`/api/v1/engine/history?range=${range}`)) as EngineHistoryPoint[];
      history.value = data || [];
    } catch (err) {
      console.warn('[Dashboard] fetchEngineHistory failed:', err);
    }
  }

  function patchDeletionProgress(data: unknown) {
    const event = data as DeletionProgress;
    const current = history.value;
    const last = current.length > 0 ? current[current.length - 1] : undefined;
    if (last) {
      history.value = [...current.slice(0, -1), { ...last, deleted: event.succeeded }];
    }
  }

  watch(dateRange, () => {
    fetchHistory();
  });

  return {
    dateRange,
    dateRangeOptions,
    dateRangeLabel,
    history,
    hasQueuedData,
    hasDeletedData,
    avgDurationMs,
    maxDurationMs,
    sparklineEChartsOption,
    durationSparklineEChartsOption,
    fetchHistory,
    patchDeletionProgress,
  };
}
