import type { SerpAPIInput, SerpAPIOutput, SerpAPIResultItem } from './types.js';

const BY_ENGINE: Record<'google' | 'amazon' | 'yelp', SerpAPIResultItem[]> = {
  google: [
    {
      title: 'Executive assistant automation examples',
      link: 'https://google.example.com/executive-assistant-automation',
      source: 'Google'
    },
    {
      title: 'Building deterministic AI workflows',
      link: 'https://google.example.com/deterministic-ai-workflows',
      source: 'Google'
    }
  ],
  amazon: [
    {
      title: 'Noise-cancelling headset',
      link: 'https://amazon.example.com/noise-cancelling-headset',
      source: 'Amazon'
    },
    {
      title: 'Standing desk lamp',
      link: 'https://amazon.example.com/standing-desk-lamp',
      source: 'Amazon'
    }
  ],
  yelp: [
    {
      title: 'Cafe Meridian',
      link: 'https://yelp.example.com/cafe-meridian',
      source: 'Yelp'
    },
    {
      title: 'Boardroom Bistro',
      link: 'https://yelp.example.com/boardroom-bistro',
      source: 'Yelp'
    }
  ]
};

export async function runClient(input: SerpAPIInput): Promise<SerpAPIOutput> {
  const engine = input.engine ?? 'google';
  const queryTerms = input.query.toLowerCase().split(/\s+/).filter((term) => term.length > 1);
  const results = BY_ENGINE[engine]
    .filter((item) => {
      const target = `${item.title} ${item.link}`.toLowerCase();
      return queryTerms.some((term) => target.includes(term));
    })
    .slice(0, input.max_results ?? 5);

  return {
    provider: 'serpapi',
    engine,
    results
  };
}
