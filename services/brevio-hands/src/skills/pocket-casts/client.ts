import type { PocketCastsEpisode, PocketCastsInput, PocketCastsOutput } from './types.js';

const QUEUE: PocketCastsEpisode[] = [
  {
    id: 'pc_001',
    title: 'Executive Workflow Deep Dive',
    source: 'youtube'
  },
  {
    id: 'pc_002',
    title: 'Ops Weekly Briefing',
    source: 'podcast_feed'
  }
];

export async function runClient(input: PocketCastsInput): Promise<PocketCastsOutput> {
  if (input.action === 'list_queue') {
    return {
      provider: 'pocket-casts',
      action: 'list_queue',
      queue: QUEUE
    };
  }

  if (input.action === 'queue_from_youtube') {
    return {
      provider: 'pocket-casts',
      action: 'queue_from_youtube',
      queued: true
    };
  }

  return {
    provider: 'pocket-casts',
    action: 'remove_episode',
    removed: true
  };
}
