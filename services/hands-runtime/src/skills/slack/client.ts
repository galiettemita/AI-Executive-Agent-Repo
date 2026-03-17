// Plan §6 steps 6–7 — Real Slack API conversations.list / chat.postMessage
// Replaces: hardcoded CHANNELS=[{id:'C0123OPS',name:'ops'}]

import type { SlackInput, SlackOutput } from './types.js';

interface SlackApiResponse {
  ok: boolean;
  error?: string;
  channels?: Array<{ id: string; name: string }>;
  channel?: string;
  ts?: string;
}

export async function runClient(input: SlackInput): Promise<SlackOutput> {
  const token = process.env.SLACK_BOT_TOKEN;
  if (!token) throw new Error('slack: SLACK_BOT_TOKEN not set');

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };

  if (input.action === 'list_channels') {
    // Plan §6 step 6: POST conversations.list, body {limit:100, exclude_archived:true}
    const response = await fetch('https://slack.com/api/conversations.list', {
      method: 'POST',
      headers,
      body: JSON.stringify({ limit: 100, exclude_archived: true }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`slack: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as SlackApiResponse;
    if (!data.ok) throw new Error(`slack: ${data.error ?? 'unknown error'}`);

    return {
      provider: 'slack',
      action: 'list_channels',
      channels: (data.channels ?? []).map((c) => ({ id: c.id, name: c.name })),
    };
  }

  if (input.action === 'post_message') {
    // Plan §6 step 7: POST chat.postMessage with channel and text
    const response = await fetch('https://slack.com/api/chat.postMessage', {
      method: 'POST',
      headers,
      body: JSON.stringify({ channel: input.channel_id, text: input.text }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`slack: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as SlackApiResponse;
    if (!data.ok) throw new Error(`slack: ${data.error ?? 'unknown error'}`);

    return {
      provider: 'slack',
      action: 'post_message',
      post: {
        channel_id: data.channel ?? input.channel_id ?? '',
        message_ts: data.ts ?? '',
        text: input.text ?? '',
      },
    };
  }

  if (input.action === 'add_reaction') {
    // reactions.add endpoint
    const response = await fetch('https://slack.com/api/reactions.add', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        channel: input.channel_id,
        timestamp: input.message_ts,
        name: input.emoji,
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`slack: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as SlackApiResponse;
    if (!data.ok) throw new Error(`slack: ${data.error ?? 'unknown error'}`);

    return {
      provider: 'slack',
      action: 'add_reaction',
      reacted: true,
    };
  }

  throw new Error(`slack: unknown action ${input.action}`);
}
