export type GeminiDeepResearchAction = 'run_research';

export interface GeminiDeepResearchInput {
  action: GeminiDeepResearchAction;
  topic?: string;
  depth?: 'standard' | 'deep';
}

export interface GeminiDeepResearchOutput {
  provider: 'gemini-deep-research';
  action: GeminiDeepResearchAction;
  report_sections: string[];
  citations: string[];
  summary: string;
}
