export type AlterActionsAction = 'list_actions' | 'trigger_action';

export interface AlterActionsInput {
  action: AlterActionsAction;
  action_key?: string;
  app_name?: string;
  parameters?: Record<string, string>;
  confirmed?: boolean;
}

export interface AlterActionDescriptor {
  action_key: string;
  app_name: string;
  display_name: string;
  callback_url_template: string;
}

export interface AlterActionsOutput {
  provider: 'alter-actions';
  action: AlterActionsAction;
  actions: AlterActionDescriptor[];
  triggered_action?: string;
  callback_url?: string;
  summary: string;
}
