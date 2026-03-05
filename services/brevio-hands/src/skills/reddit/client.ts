import type { RedditInput, RedditOutput, RedditPostSummary } from './types.js';

const POSTS: RedditPostSummary[] = [
  {
    id: 't3_ops001',
    subreddit: 'operations',
    title: 'How do you run weekly executive reviews?',
    score: 421,
    url: 'https://reddit.example.com/r/operations/t3_ops001'
  },
  {
    id: 't3_ai002',
    subreddit: 'artificial',
    title: 'Prompt reliability patterns in production systems',
    score: 318,
    url: 'https://reddit.example.com/r/artificial/t3_ai002'
  }
];

export async function runClient(input: RedditInput): Promise<RedditOutput> {
  if (input.action === 'list_hot') {
    return {
      provider: 'reddit',
      action: 'list_hot',
      posts: POSTS
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const posts = POSTS.filter((post) => {
      const haystack = `${post.title} ${post.subreddit}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });
    return {
      provider: 'reddit',
      action: 'search',
      posts
    };
  }

  return {
    provider: 'reddit',
    action: 'post',
    submitted: true,
    post_id: 't3_newpost001'
  };
}
