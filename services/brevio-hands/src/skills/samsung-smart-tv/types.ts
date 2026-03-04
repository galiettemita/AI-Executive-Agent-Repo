export type SamsungSmartTVAction = 'power_on' | 'power_off' | 'launch_app' | 'set_volume' | 'status';

export interface SamsungSmartTVInput {
  action: SamsungSmartTVAction;
  device_id?: string;
  app_id?: string;
  volume_pct?: number;
}

export interface SamsungSmartTVOutput {
  provider: 'samsung-smart-tv';
  action: SamsungSmartTVAction;
  device_id: string;
  power_state: 'on' | 'off';
  current_app: string;
  volume_pct: number;
  summary: string;
}
