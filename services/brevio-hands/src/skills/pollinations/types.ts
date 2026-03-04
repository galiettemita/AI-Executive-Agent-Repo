export type PollinationsAction = 'generate_image' | 'generate_video' | 'generate_audio';

export interface PollinationsInput {
  action: PollinationsAction;
  prompt?: string;
  model?: string;
  size?: string;
}

export interface PollinationsOutput {
  provider: 'pollinations';
  action: PollinationsAction;
  asset_url: string;
  model_used: string;
  summary: string;
}
