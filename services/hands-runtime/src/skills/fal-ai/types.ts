export interface FalAIInput {
  prompt: string;
  model?: string;
  size?: 'square' | 'portrait' | 'landscape';
}

export interface FalAIOutput {
  provider: 'fal-ai';
  image_url: string;
  model_used: string;
  size: 'square' | 'portrait' | 'landscape';
}
