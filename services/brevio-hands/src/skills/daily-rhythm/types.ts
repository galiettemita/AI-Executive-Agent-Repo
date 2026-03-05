export type DailyRhythmAction = 'compose_briefing' | 'wind_down_prompt';

export type DailyRhythmPriority = 'high' | 'medium' | 'low';

export interface DailyRhythmTask {
  title: string;
  due_time_local?: string;
  priority: DailyRhythmPriority;
  estimated_minutes: number;
}

export interface DailyRhythmMeeting {
  title: string;
  start_local: string;
  end_local: string;
}

export interface DailyRhythmScheduleBlock {
  title: string;
  start_local: string;
  end_local: string;
  kind: 'focus' | 'meeting' | 'admin' | 'recovery';
}

export interface DailyRhythmInput {
  action: DailyRhythmAction;
  timezone: string;
  date: string;
  wake_time_local?: string;
  tasks?: DailyRhythmTask[];
  meetings?: DailyRhythmMeeting[];
  weather_summary?: string;
  energy_level?: 'low' | 'steady' | 'high';
}

export interface DailyRhythmOutput {
  provider: 'daily-rhythm';
  action: DailyRhythmAction;
  briefing_text: string;
  priorities: string[];
  schedule_blocks: DailyRhythmScheduleBlock[];
  nudges: string[];
}
