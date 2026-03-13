import type { AlterActionDescriptor, AlterActionsInput, AlterActionsOutput } from './types.js';

const ACTIONS: AlterActionDescriptor[] = [
  {
    action_key: 'bear.new_note',
    app_name: 'Bear',
    display_name: 'Create Bear Note',
    callback_url_template: 'bear://x-callback-url/create?title={title}&text={text}'
  },
  {
    action_key: 'things.new_todo',
    app_name: 'Things',
    display_name: 'Create Things Todo',
    callback_url_template: 'things:///add?title={title}'
  },
  {
    action_key: 'obsidian.open_note',
    app_name: 'Obsidian',
    display_name: 'Open Obsidian Note',
    callback_url_template: 'obsidian://open?vault={vault}&file={file}'
  }
];

function applyTemplate(template: string, params: Record<string, string>): string {
  return Object.entries(params).reduce((acc, [key, value]) => {
    return acc.replaceAll(`{${key}}`, encodeURIComponent(value));
  }, template);
}

export async function runClient(input: AlterActionsInput): Promise<AlterActionsOutput> {
  if (input.action === 'list_actions') {
    return {
      provider: 'alter-actions',
      action: input.action,
      actions: ACTIONS,
      summary: `Listed ${ACTIONS.length} configured local app action(s).`
    };
  }

  const selected: AlterActionDescriptor =
    ACTIONS.find((item) => item.action_key === input.action_key) ??
    (ACTIONS[0] as AlterActionDescriptor);
  const callback_url = applyTemplate(selected.callback_url_template, input.parameters ?? {});

  return {
    provider: 'alter-actions',
    action: input.action,
    actions: ACTIONS,
    triggered_action: selected.action_key,
    callback_url,
    summary: `Prepared callback URL for ${selected.display_name}.`
  };
}
