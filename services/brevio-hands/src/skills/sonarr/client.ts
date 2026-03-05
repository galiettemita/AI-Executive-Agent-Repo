import type { SonarrInput, SonarrOutput, SonarrSeries } from './types.js';

const BASE_QUEUE: SonarrSeries[] = [
  {
    series_id: 'tvdb-81189',
    title: 'Breaking Bad',
    status: 'queued',
    quality_profile: 'HD-1080p'
  },
  {
    series_id: 'tvdb-121361',
    title: 'Westworld',
    status: 'monitored',
    quality_profile: '4K-UHD'
  }
];

export async function runClient(input: SonarrInput): Promise<SonarrOutput> {
  if (input.action === 'list_queue') {
    return {
      provider: 'sonarr',
      action: input.action,
      series: BASE_QUEUE,
      queue_count: BASE_QUEUE.length,
      summary: `Sonarr queue currently tracks ${BASE_QUEUE.length} series.`
    };
  }

  if (input.action === 'search_series') {
    return {
      provider: 'sonarr',
      action: input.action,
      series: [
        {
          series_id: 'tvdb-search',
          title: input.query ?? 'Unknown series',
          status: 'queued',
          quality_profile: input.quality_profile ?? 'HD-1080p'
        }
      ],
      queue_count: 1,
      summary: `Found 1 series candidate for "${input.query}".`
    };
  }

  return {
    provider: 'sonarr',
    action: input.action,
    series: [
      {
        series_id: input.tvdb_id ?? 'tvdb-unknown',
        title: 'Added series',
        status: 'monitored',
        quality_profile: input.quality_profile ?? 'HD-1080p'
      }
    ],
    queue_count: 1,
    summary: `Added series ${input.tvdb_id} to Sonarr monitoring.`
  };
}
