export type LinearAction = 'issue_list' | 'issue_create' | 'issue_update';

export interface LinearInput {
  action: LinearAction;
  team_id?: string;
  issue_id?: string;
  title?: string;
  description?: string;
  status?: string;
}

export interface LinearIssue {
  issue_id: string;
  title: string;
  status: string;
  team_id: string;
}

export interface LinearOutput {
  provider: 'linear';
  action: LinearAction;
  issue_id?: string;
  issues?: LinearIssue[];
}
