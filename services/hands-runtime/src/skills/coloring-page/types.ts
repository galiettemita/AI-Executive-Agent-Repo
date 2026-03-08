export type ColoringPageAction = 'generate_from_prompt' | 'generate_from_image';

export interface ColoringPageInput {
  action: ColoringPageAction;
  prompt?: string;
  image_url?: string;
  complexity?: 'easy' | 'medium' | 'advanced';
}

export interface ColoringPageOutput {
  provider: 'coloring-page';
  action: ColoringPageAction;
  output_url: string;
  page_size: 'A4' | 'Letter';
  line_density: 'low' | 'medium' | 'high';
  summary: string;
}
