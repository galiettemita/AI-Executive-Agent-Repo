import type { PlanMyDayInput, PlanMyDayOutput, PlanMyDayTask, PlanMyDayTimeBlock } from './types.js';

function toMinutes(value: string): number {
  const parts = value.split(':').map((part) => Number(part));
  const hours = parts[0] ?? 0;
  const minutes = parts[1] ?? 0;
  return hours * 60 + minutes;
}

function toClock(minutesTotal: number): string {
  const hours = Math.floor(minutesTotal / 60) % 24;
  const minutes = minutesTotal % 60;
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
}

function sortedTasks(tasks: PlanMyDayTask[]): PlanMyDayTask[] {
  const weight: Record<PlanMyDayTask['priority'], number> = {
    high: 3,
    medium: 2,
    low: 1
  };

  return [...tasks].sort((left, right) => {
    const priorityDiff = weight[right.priority] - weight[left.priority];
    if (priorityDiff !== 0) {
      return priorityDiff;
    }

    if (left.energy !== right.energy) {
      return left.energy === 'deep' ? -1 : 1;
    }

    return right.duration_minutes - left.duration_minutes;
  });
}

function allocateBlocks(input: PlanMyDayInput): { time_blocks: PlanMyDayTimeBlock[]; overflow_tasks: string[] } {
  const tasks = sortedTasks(input.tasks ?? []);
  const windows = input.available_windows ?? [
    { start_local: '09:00', end_local: '12:00' },
    { start_local: '13:00', end_local: '17:00' }
  ];

  const blocks: PlanMyDayTimeBlock[] = [];
  const overflow: string[] = [];
  let taskIndex = 0;

  for (const window of windows) {
    let cursor = toMinutes(window.start_local);
    const windowEnd = toMinutes(window.end_local);

    while (taskIndex < tasks.length) {
      const task = tasks[taskIndex] as PlanMyDayTask;
      if (cursor + task.duration_minutes > windowEnd) {
        break;
      }

      const start = cursor;
      const end = cursor + task.duration_minutes;
      blocks.push({
        title: task.title,
        start_local: toClock(start),
        end_local: toClock(end),
        source: 'task'
      });

      cursor = end;
      taskIndex += 1;

      if (cursor + 10 <= windowEnd) {
        blocks.push({
          title: 'Recovery break',
          start_local: toClock(cursor),
          end_local: toClock(cursor + 10),
          source: 'break'
        });
        cursor += 10;
      }
    }

    if (cursor < windowEnd) {
      blocks.push({
        title: 'Buffer',
        start_local: toClock(cursor),
        end_local: toClock(windowEnd),
        source: 'buffer'
      });
    }
  }

  for (; taskIndex < tasks.length; taskIndex += 1) {
    overflow.push((tasks[taskIndex] as PlanMyDayTask).title);
  }

  return { time_blocks: blocks.slice(0, 30), overflow_tasks: overflow.slice(0, 20) };
}

export async function runClient(input: PlanMyDayInput): Promise<PlanMyDayOutput> {
  const { time_blocks, overflow_tasks } = allocateBlocks(input);
  const strategy_notes = [
    'Put deep work before communication-heavy tasks.',
    'Keep at least one buffer block for unpredictability.',
    'Batch lightweight admin tasks after a focus sprint.'
  ];

  if (input.action === 'rebalance_plan') {
    return {
      provider: 'plan-my-day',
      action: input.action,
      time_blocks,
      overflow_tasks,
      strategy_notes: [
        `Rebalanced for disruptions: ${(input.disruptions ?? []).join(', ')}.`,
        ...strategy_notes
      ]
    };
  }

  return {
    provider: 'plan-my-day',
    action: input.action,
    time_blocks,
    overflow_tasks,
    strategy_notes
  };
}
