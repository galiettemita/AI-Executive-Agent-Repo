import type { BlueskyInput, BlueskyOutput, BlueskyPost } from './types.js';

const TIMELINE: BlueskyPost[] = [
  {
    uri: 'at://did:plc:team/post/1',
    author_handle: 'ops-team.bsky.social',
    text: 'Operational update: all systems nominal.',
    like_count: 28
  },
  {
    uri: 'at://did:plc:founder/post/2',
    author_handle: 'founder.bsky.social',
    text: 'New thoughts on executive workflow automation.',
    like_count: 41
  }
];

export async function runClient(input: BlueskyInput): Promise<BlueskyOutput> {
  if (input.action === 'timeline') {
    return {
      provider: 'bluesky',
      action: 'timeline',
      posts: TIMELINE
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const posts = TIMELINE.filter((post) => {
      const haystack = `${post.author_handle} ${post.text}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });

    return {
      provider: 'bluesky',
      action: 'search',
      posts
    };
  }

  return {
    provider: 'bluesky',
    action: 'post',
    posted: true,
    uri: 'at://did:plc:exec/post/100'
  };
}
