import type { PostAtInput, PostAtOutput } from './types.js';

export async function runClient(input: PostAtInput): Promise<PostAtOutput> {
  const checkpoints = [
    {
      timestamp: '2026-03-04T08:00:00.000Z',
      location: 'Vienna Distribution Center',
      status: 'In transit'
    },
    {
      timestamp: '2026-03-04T14:30:00.000Z',
      location: 'Graz Hub',
      status: 'Out for local processing'
    }
  ];

  return {
    provider: 'post-at',
    action: 'track_parcel',
    tracking_number: input.tracking_number ?? 'UNKNOWN',
    latest_status: checkpoints[1]?.status ?? 'Unknown',
    checkpoints,
    summary: `Retrieved ${checkpoints.length} Austrian Post checkpoints.`
  };
}
