import type { AppleMediaDevice, AppleMediaInput, AppleMediaOutput } from './types.js';

const DEVICES: AppleMediaDevice[] = [
  { device_name: 'Living Room Apple TV', device_type: 'apple_tv', is_active: true },
  { device_name: 'Kitchen HomePod', device_type: 'homepod', is_active: true },
  { device_name: 'Office AirPlay Speaker', device_type: 'airplay_speaker', is_active: false }
];

function playbackState(volumePct: number): NonNullable<AppleMediaOutput['now_playing']> {
  return {
    title: 'Late Night Focus',
    artist: 'Brevio Sessions',
    position_seconds: 138,
    is_playing: true,
    volume_pct: volumePct
  };
}

export async function runClient(input: AppleMediaInput): Promise<AppleMediaOutput> {
  if (input.action === 'discover_devices') {
    return {
      provider: 'apple-media',
      action: input.action,
      devices: DEVICES,
      summary: `Discovered ${DEVICES.length} Apple media endpoint(s).`
    };
  }

  if (input.action === 'playback_status') {
    return {
      provider: 'apple-media',
      action: input.action,
      devices: DEVICES,
      now_playing: playbackState(42),
      summary: `Playback status fetched for ${input.device_name}.`
    };
  }

  const volume = input.command === 'set_volume' ? (input.volume_pct ?? 40) : 40;
  return {
    provider: 'apple-media',
    action: input.action,
    devices: DEVICES,
    now_playing: playbackState(volume),
    applied_command: input.command,
    summary: `Applied ${input.command} on ${input.device_name}.`
  };
}
