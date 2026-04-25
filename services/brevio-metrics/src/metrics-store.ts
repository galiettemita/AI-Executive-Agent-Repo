import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import path from 'node:path';

export interface HistogramSeries {
  count: number;
  sum: number;
  buckets: Map<number, number>;
}

interface HistogramSnapshot {
  count: number;
  sum: number;
  buckets: Array<[number, number]>;
}

interface MetricsSnapshot {
  version: 1;
  counters: Array<[string, number]>;
  gauges: Array<[string, number]>;
  histograms: Array<[string, HistogramSnapshot]>;
}

function cloneHistogram(series: HistogramSeries): HistogramSeries {
  return {
    count: series.count,
    sum: series.sum,
    buckets: new Map(series.buckets.entries())
  };
}

export class MetricsStore {
  private readonly counters: Map<string, number>;
  private readonly gauges: Map<string, number>;
  private readonly histograms: Map<string, HistogramSeries>;
  private readonly filePath?: string;

  constructor(filePath?: string) {
    this.filePath = filePath;
    const snapshot = this.loadSnapshot();
    this.counters = snapshot.counters;
    this.gauges = snapshot.gauges;
    this.histograms = snapshot.histograms;
  }

  mode(): 'in_memory' | 'local_file_snapshot' {
    return this.filePath ? 'local_file_snapshot' : 'in_memory';
  }

  snapshotPath(): string | undefined {
    return this.filePath;
  }

  stats(): { counters: number; gauges: number; histograms: number } {
    return {
      counters: this.counters.size,
      gauges: this.gauges.size,
      histograms: this.histograms.size
    };
  }

  incrementCounter(key: string, delta: number): number {
    const next = (this.counters.get(key) ?? 0) + delta;
    this.counters.set(key, next);
    this.persist();
    return next;
  }

  setGauge(key: string, value: number): number {
    this.gauges.set(key, value);
    this.persist();
    return value;
  }

  observeHistogram(key: string, value: number, buckets: readonly number[]): HistogramSeries {
    let series = this.histograms.get(key);
    if (!series) {
      series = {
        count: 0,
        sum: 0,
        buckets: new Map()
      };
      for (const bucket of buckets) {
        series.buckets.set(bucket, 0);
      }
      this.histograms.set(key, series);
    }

    series.count += 1;
    series.sum += value;
    for (const bucket of buckets) {
      if (value <= bucket) {
        series.buckets.set(bucket, (series.buckets.get(bucket) ?? 0) + 1);
      }
    }
    this.persist();
    return cloneHistogram(series);
  }

  counterEntries(): Array<[string, number]> {
    return Array.from(this.counters.entries());
  }

  gaugeEntries(): Array<[string, number]> {
    return Array.from(this.gauges.entries());
  }

  histogramEntries(): Array<[string, HistogramSeries]> {
    return Array.from(this.histograms.entries()).map(([key, value]) => [key, cloneHistogram(value)]);
  }

  snapshot(): Record<string, unknown> {
    return {
      counters: this.counterEntries().map(([key, value]) => ({ key, value })),
      gauges: this.gaugeEntries().map(([key, value]) => ({ key, value })),
      histograms: this.histogramEntries().map(([key, value]) => ({
        key,
        count: value.count,
        sum: value.sum,
        buckets: Array.from(value.buckets.entries()).map(([bucket, count]) => ({ bucket, count }))
      }))
    };
  }

  private loadSnapshot(): {
    counters: Map<string, number>;
    gauges: Map<string, number>;
    histograms: Map<string, HistogramSeries>;
  } {
    if (!this.filePath || !existsSync(this.filePath)) {
      return {
        counters: new Map(),
        gauges: new Map(),
        histograms: new Map()
      };
    }

    try {
      const raw = readFileSync(this.filePath, 'utf8');
      const parsed = JSON.parse(raw) as Partial<MetricsSnapshot>;
      if (!parsed || typeof parsed !== 'object') {
        throw new Error('snapshot must be a JSON object');
      }
      if ('version' in parsed && parsed.version !== 1) {
        throw new Error(`unsupported snapshot version: ${String(parsed.version)}`);
      }

      return {
        counters: this.parseNumericMap(parsed.counters, 'counters'),
        gauges: this.parseNumericMap(parsed.gauges, 'gauges'),
        histograms: this.parseHistograms(parsed.histograms)
      };
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`metrics snapshot is corrupt at ${this.filePath}: ${detail}`);
    }
  }

  private parseNumericMap(value: unknown, label: string): Map<string, number> {
    if (value === undefined) {
      return new Map();
    }
    if (!Array.isArray(value)) {
      throw new Error(`snapshot ${label} must be an array`);
    }
    const out = new Map<string, number>();
    for (const entry of value) {
      if (!Array.isArray(entry) || entry.length !== 2 || typeof entry[0] !== 'string' || !entry[0].trim() || typeof entry[1] !== 'number' || !Number.isFinite(entry[1])) {
        throw new Error(`snapshot ${label} entry is invalid`);
      }
      out.set(entry[0], entry[1]);
    }
    return out;
  }

  private parseHistograms(value: unknown): Map<string, HistogramSeries> {
    if (value === undefined) {
      return new Map();
    }
    if (!Array.isArray(value)) {
      throw new Error('snapshot histograms must be an array');
    }
    const out = new Map<string, HistogramSeries>();
    for (const entry of value) {
      if (!Array.isArray(entry) || entry.length !== 2 || typeof entry[0] !== 'string' || !entry[0].trim() || !entry[1] || typeof entry[1] !== 'object') {
        throw new Error('snapshot histogram entry is invalid');
      }
      const series = entry[1] as Partial<HistogramSnapshot>;
      if (typeof series.count !== 'number' || !Number.isFinite(series.count) || typeof series.sum !== 'number' || !Number.isFinite(series.sum) || !Array.isArray(series.buckets)) {
        throw new Error('snapshot histogram payload is invalid');
      }
      const buckets = new Map<number, number>();
      for (const bucketEntry of series.buckets) {
        if (!Array.isArray(bucketEntry) || bucketEntry.length !== 2 || typeof bucketEntry[0] !== 'number' || !Number.isFinite(bucketEntry[0]) || typeof bucketEntry[1] !== 'number' || !Number.isFinite(bucketEntry[1])) {
          throw new Error('snapshot histogram bucket is invalid');
        }
        buckets.set(bucketEntry[0], bucketEntry[1]);
      }
      out.set(entry[0], {
        count: series.count,
        sum: series.sum,
        buckets
      });
    }
    return out;
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }

    mkdirSync(path.dirname(this.filePath), { recursive: true });
    const tmpPath = `${this.filePath}.${process.pid}.tmp`;
    const snapshot: MetricsSnapshot = {
      version: 1,
      counters: this.counterEntries(),
      gauges: this.gaugeEntries(),
      histograms: this.histogramEntries().map(([key, value]) => [
        key,
        {
          count: value.count,
          sum: value.sum,
          buckets: Array.from(value.buckets.entries())
        }
      ])
    };
    writeFileSync(tmpPath, JSON.stringify(snapshot, null, 2), 'utf8');
    renameSync(tmpPath, this.filePath);
  }
}
