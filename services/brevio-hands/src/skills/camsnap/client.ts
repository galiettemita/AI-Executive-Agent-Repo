import type { CamsnapInput, CamsnapOutput } from './types.js';

export async function runClient(input: CamsnapInput): Promise<CamsnapOutput> {
  const extension = input.action === 'capture_clip' ? 'mp4' : 'jpg';
  return {
    provider: 'camsnap',
    action: input.action,
    media_url: `https://assets.brevio.local/camsnap/${input.camera_id}.${extension}`,
    captured_at_utc: '2026-03-04T18:00:00.000Z',
    resolution: '1920x1080',
    summary: `Captured ${input.action === 'capture_clip' ? 'clip' : 'frame'} from camera ${input.camera_id}.`
  };
}
