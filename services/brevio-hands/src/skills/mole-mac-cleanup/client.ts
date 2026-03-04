import type { MoleMacCleanupInput, MoleMacCleanupOutput } from './types.js';

export async function runClient(input: MoleMacCleanupInput): Promise<MoleMacCleanupOutput> {
  const reclaimable = input.mode === 'deep' ? 4800 : 1200;
  return {
    provider: 'mole-mac-cleanup',
    action: input.action,
    reclaimable_mb: reclaimable,
    cleaned_mb: input.action === 'run_cleanup' ? Math.round(reclaimable * 0.65) : undefined,
    categories: ['Caches', 'Log files', 'Temporary artifacts'],
    summary: `Cleanup ${input.action} completed in ${input.mode ?? 'quick'} mode.`
  };
}
