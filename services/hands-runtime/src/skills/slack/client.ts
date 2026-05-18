import type { SlackChannel, SlackInput, SlackOutput } from './types.js';

const CHANNELS: SlackChannel[] = [
  { id: 'C0123OPS', name: 'ops' },
  { id: 'C0456EXEC', name: 'executive-staff' },
  { id: 'C0789ENG', name: 'engineering' }
];

export async function runClient(input: SlackInput): Promise<SlackOutput> {
  if (input.action === 'list_channels') {
    return {
      provider: 'slack',
      action: 'list_channels',
      channels: CHANNELS
    };
  }

  if (input.action === 'post_message') {
    return {
      provider: 'slack',
      action: 'post_message',
      post: {
        channel_id: input.channel_id ?? 'C0123OPS',
        message_ts: '1710000000.000100',
        text: input.text ?? ''
      }
    };
  }

  return {
    provider: 'slack',
    action: 'add_reaction',
    reacted: true
  };
}
