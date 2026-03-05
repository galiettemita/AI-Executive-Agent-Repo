export type CamsnapAction = 'capture_frame' | 'capture_clip';

export interface CamsnapInput {
  action: CamsnapAction;
  camera_id?: string;
  duration_seconds?: number;
}

export interface CamsnapOutput {
  provider: 'camsnap';
  action: CamsnapAction;
  media_url: string;
  captured_at_utc: string;
  resolution: string;
  summary: string;
}
