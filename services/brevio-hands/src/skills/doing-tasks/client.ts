import type { DoingTasksInput, DoingTasksOutput } from './types.js';

export async function runClient(input: DoingTasksInput): Promise<DoingTasksOutput> {
  const routedSkill = input.skill_hint ?? 'todo';
  const plan = ['Classify task intent', 'Select execution skill', 'Dispatch and track completion'];

  return {
    provider: 'doing-tasks',
    action: input.action,
    routed_skill: routedSkill,
    execution_plan: plan,
    summary: `Orchestrated task flow using ${routedSkill}.`
  };
}
