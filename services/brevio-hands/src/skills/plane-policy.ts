import type { ISkillAdapter } from '@brevio/shared';

export function isHandsExecutableAdapter(
  adapter: Pick<ISkillAdapter, 'plane'> | null | undefined
): adapter is Pick<ISkillAdapter, 'plane'> & { plane: 'hands' | 'gateway' } {
  return adapter?.plane === 'hands' || adapter?.plane === 'gateway';
}
