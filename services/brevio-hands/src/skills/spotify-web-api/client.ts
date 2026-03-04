import type { SpotifyTrack, SpotifyWebInput, SpotifyWebOutput } from './types.js';

const TRACKS: SpotifyTrack[] = [
  {
    track_id: 'trk_001',
    name: 'Northbound Focus',
    artist: 'Lumen Field'
  },
  {
    track_id: 'trk_002',
    name: 'Deep Work Pulse',
    artist: 'Exec Ensemble'
  },
  {
    track_id: 'trk_003',
    name: 'Midnight Strategy',
    artist: 'Atlas Avenue'
  }
];

export async function runClient(input: SpotifyWebInput): Promise<SpotifyWebOutput> {
  if (input.action === 'playback') {
    return {
      provider: 'spotify-web-api',
      action: 'playback',
      playing: {
        track_id: 'trk_002',
        name: 'Deep Work Pulse',
        artist: 'Exec Ensemble',
        progress_ms: 68214
      }
    };
  }

  if (input.action === 'search') {
    const query = input.query?.toLowerCase() ?? '';
    return {
      provider: 'spotify-web-api',
      action: 'search',
      results: TRACKS.filter((track) => `${track.name} ${track.artist}`.toLowerCase().includes(query))
    };
  }

  if (input.action === 'history') {
    return {
      provider: 'spotify-web-api',
      action: 'history',
      results: [TRACKS[1], TRACKS[0]]
    };
  }

  return {
    provider: 'spotify-web-api',
    action: 'top_tracks',
    results: [TRACKS[1], TRACKS[2], TRACKS[0]]
  };
}
