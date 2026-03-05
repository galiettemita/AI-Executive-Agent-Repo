export interface PerplexityInput {
  query: string;
  model?: string;
}

export interface PerplexityCitation {
  title: string;
  url: string;
}

export interface PerplexityOutput {
  provider: 'perplexity';
  answer: string;
  citations: PerplexityCitation[];
}
