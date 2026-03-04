export type MorningManifestoAction = 'generate_manifesto' | 'sync_actions';

export interface MorningManifestoInput {
  action: MorningManifestoAction;
  goals?: string[];
  gratitude?: string[];
  blockers?: string[];
  tone?: 'direct' | 'supportive';
  sync_targets?: Array<'apple_reminders' | 'linear' | 'obsidian'>;
}

export interface MorningManifestoOutput {
  provider: 'morning-manifesto';
  action: MorningManifestoAction;
  manifesto: string;
  affirmations: string[];
  action_items: string[];
  sync_targets: string[];
}
