export type VeoAction = 'generate_video' | 'check_status';

export interface VeoInput {
  action: VeoAction;
  prompt?: string;
  job_id?: string;
}

export interface VeoOutput {
  provider: 'veo';
  action: VeoAction;
  job_id: string;
  status: 'queued' | 'processing' | 'completed';
  video_url?: string;
  summary: string;
}
