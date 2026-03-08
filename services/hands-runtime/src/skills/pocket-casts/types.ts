export interface PocketCastsInput {
  action: 'queue_from_youtube' | 'list_queue' | 'remove_episode';
  youtube_url?: string;
  episode_id?: string;
}

export interface PocketCastsEpisode {
  id: string;
  title: string;
  source: string;
}

export interface PocketCastsOutput {
  provider: 'pocket-casts';
  action: PocketCastsInput['action'];
  queue?: PocketCastsEpisode[];
  queued?: boolean;
  removed?: boolean;
}
