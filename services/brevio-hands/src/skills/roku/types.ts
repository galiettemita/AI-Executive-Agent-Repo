export type RokuAction = 'launch_app' | 'key_press' | 'status';

export interface RokuInput {
  action: RokuAction;
  device_id?: string;
  app_id?: string;
  key?: string;
}

export interface RokuOutput {
  provider: 'roku';
  action: RokuAction;
  device_id: string;
  current_app: string;
  power_state: 'on' | 'off';
  summary: string;
}
