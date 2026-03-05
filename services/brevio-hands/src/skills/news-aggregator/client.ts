import type { NewsAggregatorInput, NewsAggregatorItem, NewsAggregatorOutput } from './types.js';

const ITEMS: NewsAggregatorItem[] = [
  {
    source: 'Hacker News',
    title: 'New workflow engines for AI assistants',
    url: 'https://news.example.com/hn-workflow-engines'
  },
  {
    source: 'Product Hunt',
    title: 'Ops dashboard startup roundup',
    url: 'https://news.example.com/ph-ops-dashboard-roundup'
  },
  {
    source: 'GitHub Trending',
    title: 'Top open-source orchestration repos this week',
    url: 'https://news.example.com/gh-trending-orchestration'
  }
];

export async function runClient(input: NewsAggregatorInput): Promise<NewsAggregatorOutput> {
  const topic = input.topic?.toLowerCase();
  const filtered = ITEMS.filter((item) => {
    if (!topic) {
      return true;
    }
    const haystack = `${item.source} ${item.title}`.toLowerCase();
    return haystack.includes(topic);
  });

  return {
    provider: 'news-aggregator',
    items: filtered.slice(0, input.max_items ?? 10)
  };
}
