import type { SpotifyHistoryInput, SpotifyHistoryOutput } from './types.js';

const TRACKS: SpotifyHistoryOutput['top_tracks'] = [
  { title: 'Deep Work Session', artist: 'Brevio Beats', play_count: 42 },
  { title: 'Calm Systems', artist: 'Logic Waves', play_count: 36 }
];

const ARTISTS: SpotifyHistoryOutput['top_artists'] = [
  { name: 'Brevio Beats', play_count: 92 },
  { name: 'Logic Waves', play_count: 71 }
];

export async function runClient(input: SpotifyHistoryInput): Promise<SpotifyHistoryOutput> {
  const limit = input.limit ?? 10;
  return {
    provider: 'spotify-history',
    action: input.action,
    top_tracks: TRACKS.slice(0, limit),
    top_artists: ARTISTS.slice(0, limit),
    total_listening_minutes: 1340,
    summary: `Compiled Spotify listening history for window ${input.window ?? '4w'}.`
  };
}
