import type { MeetingActionItem, MeetingAutopilotInput, MeetingAutopilotOutput } from './types.js';

function splitSentences(transcript: string): string[] {
  return transcript
    .split(/(?<=[.!?])\s+/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function inferActionItems(transcript: string, participants: string[]): MeetingActionItem[] {
  const explicit = transcript
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.toUpperCase().startsWith('ACTION:'))
    .map((line, index) => {
      const owner = participants[index % Math.max(participants.length, 1)] ?? 'Owner TBD';
      return {
        owner,
        task: line.replace(/^ACTION:\s*/i, '').slice(0, 220)
      };
    });

  if (explicit.length > 0) {
    return explicit.slice(0, 20);
  }

  const fallbackOwner = participants[0] ?? 'Owner TBD';
  return [
    { owner: fallbackOwner, task: 'Publish meeting recap with decisions within 24 hours.' },
    { owner: fallbackOwner, task: 'Track open risks and confirm owners for each by next standup.' }
  ];
}

export async function runClient(input: MeetingAutopilotInput): Promise<MeetingAutopilotOutput> {
  const transcript = input.transcript ?? '';
  const sentences = splitSentences(transcript);
  const summary = sentences.slice(0, 3).join(' ') || 'Meeting discussed updates, constraints, and next steps.';

  const decisions = sentences
    .filter((sentence) => /decided|decision|agree(d)?/i.test(sentence))
    .slice(0, 10)
    .map((sentence) => sentence.slice(0, 220));

  const participants = input.participants ?? [];
  const action_items = inferActionItems(transcript, participants);

  if (input.action === 'draft_follow_up') {
    const recipientLine = participants.join(', ');
    return {
      provider: 'meeting-autopilot',
      action: input.action,
      summary,
      decisions,
      action_items,
      follow_up_email: `Subject: ${input.meeting_title ?? 'Meeting'} follow-up\n\nHi ${recipientLine},\n\nSummary: ${summary}\n\nDecisions:\n- ${decisions.join('\n- ') || 'No explicit decisions captured.'}\n\nAction items:\n- ${action_items.map((item) => `${item.owner}: ${item.task}`).join('\n- ')}\n\nThanks.`
    };
  }

  return {
    provider: 'meeting-autopilot',
    action: input.action,
    summary,
    decisions,
    action_items
  };
}
