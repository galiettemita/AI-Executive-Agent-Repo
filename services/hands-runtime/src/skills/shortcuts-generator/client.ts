import type { ShortcutSummary, ShortcutsGeneratorInput, ShortcutsGeneratorOutput } from './types.js';

const BASE_SHORTCUTS: ShortcutSummary[] = [
  {
    shortcut_id: 'sc-morning-brief',
    name: 'Morning Brief',
    status: 'generated',
    install_url: 'https://shortcuts.example/install/sc-morning-brief',
    step_count: 6
  },
  {
    shortcut_id: 'sc-focus-lock',
    name: 'Focus Lock',
    status: 'installed',
    install_url: 'https://shortcuts.example/install/sc-focus-lock',
    step_count: 4
  }
];

function toShortcutID(name: string): string {
  return `sc-${name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '').slice(0, 20) || 'generated'}`;
}

export async function runClient(input: ShortcutsGeneratorInput): Promise<ShortcutsGeneratorOutput> {
  if (input.action === 'list_shortcuts') {
    return {
      provider: 'shortcuts-generator',
      action: input.action,
      shortcuts: BASE_SHORTCUTS,
      summary: `Found ${BASE_SHORTCUTS.length} shortcuts in local library.`
    };
  }

  if (input.action === 'generate_shortcut') {
    const generated: ShortcutSummary = {
      shortcut_id: toShortcutID(input.shortcut_name ?? 'generated-shortcut'),
      name: input.shortcut_name ?? 'Generated Shortcut',
      status: 'generated',
      install_url: `https://shortcuts.example/install/${toShortcutID(input.shortcut_name ?? 'generated-shortcut')}`,
      step_count: input.steps?.length ?? 0
    };

    return {
      provider: 'shortcuts-generator',
      action: input.action,
      shortcuts: [generated],
      summary: `Generated shortcut "${generated.name}" with ${generated.step_count} steps.`
    };
  }

  return {
    provider: 'shortcuts-generator',
    action: input.action,
    shortcuts: [
      {
        shortcut_id: input.shortcut_id ?? 'sc-unknown',
        name: 'Installed Shortcut',
        status: 'installed',
        install_url: `https://shortcuts.example/install/${input.shortcut_id ?? 'sc-unknown'}`,
        step_count: 5
      }
    ],
    summary: `Installed shortcut ${input.shortcut_id}.`
  };
}
