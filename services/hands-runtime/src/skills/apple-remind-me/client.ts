import type { AppleRemindMeInput, AppleRemindMeOutput, AppleReminder } from './types.js';

const BASE_REMINDERS: AppleReminder[] = [
  {
    reminder_id: 'rem-standup',
    title: 'Daily standup prep',
    due_at: '2026-03-05T14:00:00.000Z',
    list: 'Work',
    status: 'open'
  },
  {
    reminder_id: 'rem-dry-cleaning',
    title: 'Pick up dry cleaning',
    due_at: '2026-03-05T22:00:00.000Z',
    list: 'Personal',
    status: 'open'
  }
];

function slugID(input: string): string {
  return `rem-${input.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '').slice(0, 20) || 'new'}`;
}

export async function runClient(input: AppleRemindMeInput): Promise<AppleRemindMeOutput> {
  if (input.action === 'list') {
    return {
      provider: 'apple-reminders',
      action: input.action,
      reminders: BASE_REMINDERS,
      summary: `Found ${BASE_REMINDERS.length} reminders across Work and Personal lists.`
    };
  }

  if (input.action === 'create') {
    const created: AppleReminder = {
      reminder_id: slugID(input.title ?? 'new-reminder'),
      title: input.title ?? 'Untitled reminder',
      due_at: input.due_at,
      list: input.list ?? 'Inbox',
      status: 'open'
    };

    return {
      provider: 'apple-reminders',
      action: input.action,
      reminders: [created],
      summary: `Created reminder "${created.title}" in ${created.list}.`
    };
  }

  if (input.action === 'complete') {
    const completed: AppleReminder = {
      reminder_id: input.reminder_id ?? 'unknown',
      title: 'Completed reminder',
      list: input.list ?? 'Inbox',
      status: 'completed'
    };

    return {
      provider: 'apple-reminders',
      action: input.action,
      reminders: [completed],
      summary: `Marked reminder ${completed.reminder_id} as completed.`
    };
  }

  return {
    provider: 'apple-reminders',
    action: input.action,
    reminders: [],
    summary: `Deleted reminder ${input.reminder_id}.`
  };
}
