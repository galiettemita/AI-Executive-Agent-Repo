export type AppleRemindMeAction = 'create' | 'list' | 'complete' | 'delete';

export interface AppleRemindMeInput {
  action: AppleRemindMeAction;
  title?: string;
  due_at?: string;
  reminder_id?: string;
  list?: string;
}

export interface AppleReminder {
  reminder_id: string;
  title: string;
  due_at?: string;
  list: string;
  status: 'open' | 'completed';
}

export interface AppleRemindMeOutput {
  provider: 'apple-reminders';
  action: AppleRemindMeAction;
  reminders: AppleReminder[];
  summary: string;
}
