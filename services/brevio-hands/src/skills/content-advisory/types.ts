export type ContentAdvisoryAction = 'evaluate_title';

export interface ContentAdvisoryInput {
  action: ContentAdvisoryAction;
  title?: string;
  media_type?: 'movie' | 'tv';
  age_target?: number;
}

export interface AdvisoryCategory {
  category: 'violence' | 'language' | 'substances' | 'sexual_content';
  level: 'none' | 'mild' | 'moderate' | 'strong';
}

export interface ContentAdvisoryOutput {
  provider: 'content-advisory';
  action: ContentAdvisoryAction;
  categories: AdvisoryCategory[];
  overall_advisory: string;
  summary: string;
}
