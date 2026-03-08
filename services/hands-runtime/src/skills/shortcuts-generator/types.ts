export type ShortcutsGeneratorAction = 'generate_shortcut' | 'list_shortcuts' | 'install_shortcut';

export interface ShortcutsGeneratorInput {
  action: ShortcutsGeneratorAction;
  shortcut_name?: string;
  description?: string;
  steps?: string[];
  shortcut_id?: string;
}

export interface ShortcutSummary {
  shortcut_id: string;
  name: string;
  status: 'generated' | 'installed';
  install_url?: string;
  step_count: number;
}

export interface ShortcutsGeneratorOutput {
  provider: 'shortcuts-generator';
  action: ShortcutsGeneratorAction;
  shortcuts: ShortcutSummary[];
  summary: string;
}
