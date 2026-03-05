export type KreaApiAction = 'generate_image' | 'upscale_image' | 'list_models';

export interface KreaApiInput {
  action: KreaApiAction;
  prompt?: string;
  image_url?: string;
  model?: string;
}

export interface KreaApiOutput {
  provider: 'krea-api';
  action: KreaApiAction;
  image_url: string;
  model: string;
  quality_score: number;
  summary: string;
}
