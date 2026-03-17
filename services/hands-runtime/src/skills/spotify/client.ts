// Plan §6 step 8 — Real Spotify Web API
// 204 No Content: pause, next, volume all return 204 — never parse as JSON

interface SkillContext { token?: string; user_id?: string; }

const SPOTIFY_BASE = 'https://api.spotify.com/v1';

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const token = ctx?.token ?? process.env.SPOTIFY_TOKEN;
  if (!token) throw new Error('Spotify token required: provide ctx.token or set SPOTIFY_TOKEN');

  async function spotifyFetch(
    method: string,
    path:   string,
    body?:  object
  ): Promise<any> {
    const res = await fetch(`${SPOTIFY_BASE}${path}`, {
      method,
      headers: {
        authorization:  `Bearer ${token}`,
        'content-type': 'application/json',
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
    if (res.status === 401) throw new Error('Spotify token expired or invalid — re-authenticate');
    if (res.status === 204) return null;
    if (!res.ok)            throw new Error(`Spotify API error: ${res.status}`);
    return res.json();
  }

  switch (input.action) {

    case 'status': {
      const [playing, player] = await Promise.all([
        spotifyFetch('GET', '/me/player/currently-playing'),
        spotifyFetch('GET', '/me/player'),
      ]);
      return {
        is_playing:     player?.is_playing    ?? false,
        track:          playing?.item?.name   ?? null,
        artist:         playing?.item?.artists?.[0]?.name ?? null,
        album:          playing?.item?.album?.name        ?? null,
        progress_ms:    playing?.progress_ms  ?? null,
        duration_ms:    playing?.item?.duration_ms        ?? null,
        device:         player?.device?.name  ?? null,
        volume_percent: player?.device?.volume_percent    ?? null,
      };
    }

    case 'play': {
      if (input.query) {
        const search = await spotifyFetch('GET', `/search?type=track&q=${encodeURIComponent(input.query)}&limit=1`);
        const uri    = search?.tracks?.items?.[0]?.uri;
        if (!uri) throw new Error(`No Spotify track found for query: "${input.query}"`);
        await spotifyFetch('PUT', '/me/player/play', { uris: [uri] });
        return { action: 'play', query: input.query, track_uri: uri, status: 'ok' };
      }
      await spotifyFetch('PUT', '/me/player/play', {});
      return { action: 'play', status: 'ok' };
    }

    case 'pause': {
      await spotifyFetch('PUT', '/me/player/pause');
      return { action: 'pause', status: 'ok' };
    }

    case 'next': {
      await spotifyFetch('POST', '/me/player/next');
      return { action: 'next', status: 'ok' };
    }

    case 'set_volume': {
      if (input.volume_percent == null) {
        throw new Error('volume_percent is required (0–100)');
      }
      const vol = Math.min(100, Math.max(0, Math.round(Number(input.volume_percent))));
      if (isNaN(vol)) throw new Error('volume_percent must be a number between 0 and 100');
      await spotifyFetch('PUT', `/me/player/volume?volume_percent=${vol}`);
      return { action: 'set_volume', volume_percent: vol, status: 'ok' };
    }

    default:
      throw new Error(`Unknown Spotify action: ${input.action}. Valid: status, play, pause, next, set_volume`);
  }
}
