import type { VeoInput, VeoOutput } from './types.js';

export async function runClient(input: VeoInput): Promise<VeoOutput> {
  const jobID = input.job_id ?? 'veo-job-001';
  const completed = input.action === 'check_status';

  return {
    provider: 'veo',
    action: input.action,
    job_id: jobID,
    status: completed ? 'completed' : 'queued',
    video_url: completed ? `https://assets.brevio.local/videos/${jobID}.mp4` : undefined,
    summary: `Veo job ${jobID} is ${completed ? 'completed' : 'queued'}.`
  };
}
