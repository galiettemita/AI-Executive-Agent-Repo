import type { SpotifyPlayerInput, SpotifyPlayerOutput } from './types.js';

const TRACKS: SpotifyPlayerOutput['tracks'] = [
  { track_id: 'trk_001', title: 'Deep Work Session', artist: 'Brevio Beats', duration_seconds: 246 },
  { track_id: 'trk_002', title: 'Morning Momentum', artist: 'Office Flow', duration_seconds: 208 }
];

export async function runClient(input: SpotifyPlayerInput): Promise<SpotifyPlayerOutput> {
  if (input.action === 'search_tracks') {
    const query = (input.query ?? '').toLowerCase();
    const tracks = TRACKS.filter(
      (track) => track.title.toLowerCase().includes(query) || track.artist.toLowerCase().includes(query)
    );

    return {
      provider: 'spotify-player',
      action: input.action,
      tracks,
      queue_length: 4,
      summary: `Found ${tracks.length} track(s) for "${input.query}".`
    };
  }

  if (input.action === 'queue_track') {
    return {
      provider: 'spotify-player',
      action: input.action,
      tracks: TRACKS.filter((track) => track.track_id === input.track_id),
      queue_length: 5,
      summary: `Queued track ${input.track_id}.`
    };
  }

  return {
    provider: 'spotify-player',
    action: input.action,
    tracks: TRACKS,
    queue_length: 4,
    summary: 'Returned current terminal playback queue snapshot.'
  };
}
