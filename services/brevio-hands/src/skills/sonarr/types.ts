export type SonarrAction = 'search_series' | 'add_series' | 'list_queue';

export interface SonarrInput {
  action: SonarrAction;
  query?: string;
  tvdb_id?: string;
  quality_profile?: string;
}

export interface SonarrSeries {
  series_id: string;
  title: string;
  status: 'queued' | 'monitored';
  quality_profile: string;
}

export interface SonarrOutput {
  provider: 'sonarr';
  action: SonarrAction;
  series: SonarrSeries[];
  queue_count: number;
  summary: string;
}
