export type JiraAction = 'issue_list' | 'issue_create' | 'issue_transition';

export interface JiraInput {
  action: JiraAction;
  project_key?: string;
  issue_key?: string;
  summary?: string;
  description?: string;
  transition_to?: string;
}

export interface JiraIssue {
  issue_key: string;
  summary: string;
  status: string;
  project_key: string;
}

export interface JiraOutput {
  provider: 'jira';
  action: JiraAction;
  issue_key?: string;
  issues?: JiraIssue[];
}
