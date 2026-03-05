import type { YouTubeInput, YouTubeOutput, YouTubeSearchResult } from './types.js';

const CATALOG: YouTubeSearchResult[] = [
  {
    video_id: 'vid_exec_weekly_01',
    title: 'How Top Executives Plan Weekly Reviews',
    channel: 'Chief of Staff Lab',
    published_at: '2026-01-20T14:00:00.000Z'
  },
  {
    video_id: 'vid_ai_brief_02',
    title: 'AI Morning Briefing Workflow Explained',
    channel: 'Ops Systems',
    published_at: '2026-02-02T09:30:00.000Z'
  },
  {
    video_id: 'vid_focus_03',
    title: 'Deep Work Sprints for Founders',
    channel: 'Productivity Foundry',
    published_at: '2026-02-14T18:45:00.000Z'
  }
];

function filterByQuery(query: string): YouTubeSearchResult[] {
  const terms = query
    .toLowerCase()
    .split(/\s+/)
    .map((term) => term.trim())
    .filter((term) => term.length > 1);

  return CATALOG.filter((item) => {
    const haystack = `${item.title} ${item.channel}`.toLowerCase();
    if (terms.length === 0) {
      return true;
    }
    return terms.some((term) => haystack.includes(term));
  });
}

export async function runClient(input: YouTubeInput): Promise<YouTubeOutput> {
  if (input.mode === 'search') {
    const query = input.query ?? '';
    return {
      provider: 'youtube',
      mode: 'search',
      results: filterByQuery(query)
    };
  }

  if (input.mode === 'channel') {
    const channelQuery = input.channel_id?.toLowerCase() ?? '';
    return {
      provider: 'youtube',
      mode: 'channel',
      results: CATALOG.filter((item) => item.channel.toLowerCase().includes(channelQuery))
    };
  }

  if (!input.video_id) {
    throw new Error('YOUTUBE_VIDEO_ID_REQUIRED');
  }

  const selected = CATALOG.find((item) => item.video_id === input.video_id);
  if (!selected) {
    return {
      provider: 'youtube',
      mode: 'transcript',
      transcript: 'Transcript unavailable for requested video.'
    };
  }

  return {
    provider: 'youtube',
    mode: 'transcript',
    transcript: `${selected.title}: concise transcript summary with key points and action items.`
  };
}
