export type MoleMacCleanupAction = 'scan_cleanup' | 'run_cleanup';

export interface MoleMacCleanupInput {
  action: MoleMacCleanupAction;
  mode?: 'quick' | 'deep';
  confirmed?: boolean;
}

export interface MoleMacCleanupOutput {
  provider: 'mole-mac-cleanup';
  action: MoleMacCleanupAction;
  reclaimable_mb: number;
  cleaned_mb?: number;
  categories: string[];
  summary: string;
}
