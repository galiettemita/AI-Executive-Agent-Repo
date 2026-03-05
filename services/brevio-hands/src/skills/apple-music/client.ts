import type { AppleMusicInput, AppleMusicOutput, AppleMusicTrack } from './types.js';

const TRACKS: AppleMusicTrack[] = [
  {
    id: 'am_track_001',
    title: 'Midnight Drive',
    artist: 'Neon Harbor',
    album: 'After Hours'
  },
  {
    id: 'am_track_002',
    title: 'Focus Line',
    artist: 'The Operators',
    album: 'Deep Work'
  }
];

export async function runClient(input: AppleMusicInput): Promise<AppleMusicOutput> {
  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const tracks = TRACKS.filter((track) => {
      const haystack = `${track.title} ${track.artist} ${track.album}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });
    return {
      provider: 'apple-music',
      action: 'search',
      tracks
    };
  }

  if (input.action === 'play') {
    const nowPlaying = TRACKS.find((track) => track.id === input.track_id) ?? TRACKS[0];
    return {
      provider: 'apple-music',
      action: 'play',
      now_playing: nowPlaying
    };
  }

  return {
    provider: 'apple-music',
    action: 'add_to_playlist',
    playlist_updated: true
  };
}
