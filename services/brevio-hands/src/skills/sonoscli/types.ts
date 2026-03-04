export type SonoscliAction = 'discover' | 'play' | 'pause' | 'set_volume' | 'group' | 'status';

export interface SonoscliInput {
  action: SonoscliAction;
  speaker_id?: string;
  query?: string;
  volume_pct?: number;
  group_with?: string;
}

export interface SonosZone {
  speaker_id: string;
  name: string;
  is_playing: boolean;
  volume_pct: number;
  group_members: string[];
}

export interface SonoscliOutput {
  provider: 'sonoscli';
  action: SonoscliAction;
  zones: SonosZone[];
  summary: string;
}
