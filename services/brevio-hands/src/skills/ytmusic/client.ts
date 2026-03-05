import type { YTMusicInput, YTMusicOutput, YTMusicTrack } from './types.js';

const TRACKS: YTMusicTrack[] = [
  {
    id: 'ytm_001',
    title: 'Calm Systems',
    artist: 'Focus Circuit'
  },
  {
    id: 'ytm_002',
    title: 'Night Terminal',
    artist: 'Index Zero'
  }
];

export async function runClient(input: YTMusicInput): Promise<YTMusicOutput> {
  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const tracks = TRACKS.filter((track) => {
      const haystack = `${track.title} ${track.artist}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });
    return {
      provider: 'ytmusic',
      action: 'search',
      tracks
    };
  }

  if (input.action === 'play') {
    return {
      provider: 'ytmusic',
      action: 'play',
      now_playing: TRACKS.find((track) => track.id === input.track_id) ?? TRACKS[0]
    };
  }

  return {
    provider: 'ytmusic',
    action: 'queue',
    queued: true
  };
}
