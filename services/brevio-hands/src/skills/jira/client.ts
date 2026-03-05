import { createHash } from 'node:crypto';

import type { JiraInput, JiraIssue, JiraOutput } from './types.js';

const ISSUES: JiraIssue[] = [
  {
    issue_key: 'OPS-101',
    summary: 'Stabilize workflow retries',
    status: 'In Progress',
    project_key: 'OPS'
  },
  {
    issue_key: 'OPS-102',
    summary: 'Improve observability tags',
    status: 'To Do',
    project_key: 'OPS'
  }
];

function issueKey(projectKey: string, summary: string): string {
  const suffix = createHash('sha256').update(summary).digest('hex').slice(0, 4).toUpperCase();
  return `${projectKey}-${suffix}`;
}

export async function runClient(input: JiraInput): Promise<JiraOutput> {
  if (input.action === 'issue_list') {
    return {
      provider: 'jira',
      action: 'issue_list',
      issues: ISSUES.filter((issue) => !input.project_key || issue.project_key === input.project_key)
    };
  }

  if (input.action === 'issue_create') {
    if (!input.project_key || !input.summary) {
      throw new Error('JIRA_CREATE_FIELDS_REQUIRED');
    }

    return {
      provider: 'jira',
      action: 'issue_create',
      issue_key: issueKey(input.project_key, input.summary),
      issues: [
        {
          issue_key: issueKey(input.project_key, input.summary),
          summary: input.summary,
          status: 'To Do',
          project_key: input.project_key
        }
      ]
    };
  }

  if (!input.issue_key || !input.transition_to) {
    throw new Error('JIRA_TRANSITION_FIELDS_REQUIRED');
  }

  return {
    provider: 'jira',
    action: 'issue_transition',
    issue_key: input.issue_key,
    issues: [
      {
        issue_key: input.issue_key,
        summary: 'Transitioned issue',
        status: input.transition_to,
        project_key: input.project_key ?? 'OPS'
      }
    ]
  };
}
