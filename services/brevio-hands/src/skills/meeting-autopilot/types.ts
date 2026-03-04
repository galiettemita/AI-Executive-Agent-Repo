export type MeetingAutopilotAction = 'summarize_meeting' | 'extract_actions' | 'draft_follow_up';

export interface MeetingAutopilotInput {
  action: MeetingAutopilotAction;
  meeting_title?: string;
  transcript?: string;
  participants?: string[];
}

export interface MeetingActionItem {
  owner: string;
  task: string;
  due_hint?: string;
}

export interface MeetingAutopilotOutput {
  provider: 'meeting-autopilot';
  action: MeetingAutopilotAction;
  summary: string;
  decisions: string[];
  action_items: MeetingActionItem[];
  follow_up_email?: string;
}
