import type { LastfmInput, LastfmOutput, LastfmTrack } from './types.js';

const TRACKS: LastfmTrack[] = [
  {
    name: 'Operational Calm',
    artist: 'Focus Circuit',
    playcount: 48
  },
  {
    name: 'Late Night Review',
    artist: 'Signal Harbor',
    playcount: 36
  }
];

export async function runClient(input: LastfmInput): Promise<LastfmOutput> {
  if (input.action === 'recent_tracks') {
    return {
      provider: 'lastfm',
      action: 'recent_tracks',
      tracks: TRACKS
    };
  }

  if (input.action === 'top_tracks') {
    return {
      provider: 'lastfm',
      action: 'top_tracks',
      tracks: TRACKS
    };
  }

  return {
    provider: 'lastfm',
    action: 'artist_summary',
    artist_summary: {
      artist: input.artist ?? 'Unknown Artist',
      listeners: 128000,
      summary: 'Consistently charting in executive-focus playlists.'
    }
  };
}
