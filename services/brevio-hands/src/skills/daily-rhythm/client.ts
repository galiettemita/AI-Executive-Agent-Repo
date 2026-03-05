import type {
  DailyRhythmInput,
  DailyRhythmOutput,
  DailyRhythmScheduleBlock,
  DailyRhythmTask
} from './types.js';

function toFocusBlocks(tasks: DailyRhythmTask[]): DailyRhythmScheduleBlock[] {
  return tasks
    .filter((task) => task.priority !== 'low')
    .slice(0, 3)
    .map((task, index) => {
      const startHour = 9 + index * 2;
      const start_local = `${String(startHour).padStart(2, '0')}:00`;
      const end_local = `${String(startHour + 1).padStart(2, '0')}:00`;
      return {
        title: task.title,
        start_local,
        end_local,
        kind: 'focus'
      };
    });
}

export async function runClient(input: DailyRhythmInput): Promise<DailyRhythmOutput> {
  const meetings = (input.meetings ?? []).map((meeting) => ({
    title: meeting.title,
    start_local: meeting.start_local,
    end_local: meeting.end_local,
    kind: 'meeting' as const
  }));

  const focusBlocks = toFocusBlocks(input.tasks ?? []);
  const schedule_blocks = [...meetings, ...focusBlocks].slice(0, 10);

  const priorities = (input.tasks ?? [])
    .filter((task) => task.priority === 'high')
    .map((task) => task.title)
    .slice(0, 3);

  if (input.action === 'wind_down_prompt') {
    return {
      provider: 'daily-rhythm',
      action: input.action,
      briefing_text: `Wrap-up for ${input.date}: close open loops, prep tomorrow's top priority, and unplug intentionally.`,
      priorities,
      schedule_blocks,
      nudges: [
        'Capture one win from today.',
        'Set your top priority for tomorrow before ending the day.',
        'Start wind-down 60 minutes before bedtime.'
      ]
    };
  }

  const weather = input.weather_summary ? ` Weather: ${input.weather_summary}.` : '';
  const energy = input.energy_level ? `Energy mode: ${input.energy_level}.` : '';

  return {
    provider: 'daily-rhythm',
    action: input.action,
    briefing_text: `Good morning. It's ${input.date} in ${input.timezone}.${weather}${energy} Start with your highest-leverage block first.`,
    priorities,
    schedule_blocks,
    nudges: [
      `Start your first deep-work block by ${input.wake_time_local ?? '08:00'}.`,
      'Batch communication after your first focus block.',
      'Protect one recovery window between meetings.'
    ]
  };
}
