export type AppleMediaAction = 'discover_devices' | 'playback_status' | 'control_playback';

export type AppleMediaCommand = 'play' | 'pause' | 'next' | 'previous' | 'set_volume';

export interface AppleMediaInput {
  action: AppleMediaAction;
  device_name?: string;
  command?: AppleMediaCommand;
  volume_pct?: number;
  source?: string;
}

export interface AppleMediaDevice {
  device_name: string;
  device_type: 'apple_tv' | 'homepod' | 'airplay_speaker';
  is_active: boolean;
}

export interface AppleMediaOutput {
  provider: 'apple-media';
  action: AppleMediaAction;
  devices: AppleMediaDevice[];
  now_playing?: {
    title: string;
    artist: string;
    position_seconds: number;
    is_playing: boolean;
    volume_pct: number;
  };
  applied_command?: AppleMediaCommand;
  summary: string;
}
