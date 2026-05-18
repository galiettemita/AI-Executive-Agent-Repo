import type { BirdInput, BirdOutput, BirdPost } from './types.js';

const POSTS: BirdPost[] = [
  {
    id: 'tw_001',
    author: '@opslead',
    text: 'Runbook refresh complete for this quarter.',
    likes: 32,
    reposts: 8
  },
  {
    id: 'tw_002',
    author: '@aieng',
    text: 'Shipping deterministic workflow checks this week.',
    likes: 64,
    reposts: 21
  }
];

export async function runClient(input: BirdInput): Promise<BirdOutput> {
  if (input.action === 'timeline') {
    return {
      provider: 'bird',
      action: 'timeline',
      posts: POSTS
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const posts = POSTS.filter((post) => {
      const haystack = `${post.author} ${post.text}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });

    return {
      provider: 'bird',
      action: 'search',
      posts
    };
  }

  return {
    provider: 'bird',
    action: 'post',
    posted: true,
    post_id: 'tw_new001'
  };
}
