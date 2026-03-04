export type ChromecastAction = 'discover_devices' | 'cast_media' | 'pause' | 'resume' | 'stop' | 'status';

export interface ChromecastInput {
  action: ChromecastAction;
  device_name?: string;
  media_url?: string;
}

export interface ChromecastDevice {
  device_name: string;
  is_active: boolean;
  last_media_url?: string;
}

export interface ChromecastOutput {
  provider: 'chromecast';
  action: ChromecastAction;
  devices: ChromecastDevice[];
  summary: string;
}
