import type { SpotifyInput, SpotifyOutput } from './types.js';

export async function runClient(input: SpotifyInput): Promise<SpotifyOutput> {
  const base = {
    track: 'Midnight Focus',
    artist: 'Brevio Beats',
    is_playing: input.action !== 'pause',
    volume_pct: input.action === 'set_volume' ? (input.volume_pct ?? 50) : 50,
    device: 'MacBook Pro'
  };

  if (input.action === 'play' && input.query) {
    base.track = input.query;
  }

  return {
    provider: 'spotify',
    action: input.action,
    now_playing: base,
    summary: `Spotify action ${input.action} applied on ${base.device}.`
  };
}
