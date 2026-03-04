import type { SonosZone, SonoscliInput, SonoscliOutput } from './types.js';

const BASE_ZONES: SonosZone[] = [
  {
    speaker_id: 'sonos-living-room',
    name: 'Living Room',
    is_playing: true,
    volume_pct: 34,
    group_members: ['Living Room', 'Kitchen']
  },
  {
    speaker_id: 'sonos-office',
    name: 'Office',
    is_playing: false,
    volume_pct: 18,
    group_members: ['Office']
  }
];

export async function runClient(input: SonoscliInput): Promise<SonoscliOutput> {
  if (input.action === 'discover' || input.action === 'status') {
    return {
      provider: 'sonoscli',
      action: input.action,
      zones: BASE_ZONES,
      summary: `Detected ${BASE_ZONES.length} Sonos zones.`
    };
  }

  return {
    provider: 'sonoscli',
    action: input.action,
    zones: [
      {
        speaker_id: input.speaker_id ?? 'sonos-living-room',
        name: 'Living Room',
        is_playing: input.action !== 'pause',
        volume_pct: input.volume_pct ?? 30,
        group_members: input.group_with ? ['Living Room', input.group_with] : ['Living Room']
      }
    ],
    summary: `Sonos action ${input.action} applied to ${input.speaker_id}.`
  };
}
