export type RadarrAction = 'search_movie' | 'add_movie' | 'list_queue';

export interface RadarrInput {
  action: RadarrAction;
  query?: string;
  tmdb_id?: string;
  quality_profile?: string;
}

export interface RadarrMovie {
  movie_id: string;
  title: string;
  status: 'queued' | 'monitored';
  quality_profile: string;
}

export interface RadarrOutput {
  provider: 'radarr';
  action: RadarrAction;
  movies: RadarrMovie[];
  queue_count: number;
  summary: string;
}
