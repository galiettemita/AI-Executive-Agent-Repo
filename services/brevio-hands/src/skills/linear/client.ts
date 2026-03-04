import { createHash } from 'node:crypto';

import type { LinearInput, LinearIssue, LinearOutput } from './types.js';

const ISSUES: LinearIssue[] = [
  {
    issue_id: 'lin_001',
    title: 'Improve onboarding funnel',
    status: 'In Progress',
    team_id: 'ENG'
  },
  {
    issue_id: 'lin_002',
    title: 'Finalize prompt quality dashboard',
    status: 'Backlog',
    team_id: 'ENG'
  }
];

function issueID(title: string): string {
  return `lin_${createHash('sha256').update(title).digest('hex').slice(0, 8)}`;
}

export async function runClient(input: LinearInput): Promise<LinearOutput> {
  if (input.action === 'issue_list') {
    return {
      provider: 'linear',
      action: 'issue_list',
      issues: ISSUES.filter((issue) => !input.team_id || issue.team_id === input.team_id)
    };
  }

  if (input.action === 'issue_create') {
    if (!input.title || !input.team_id) {
      throw new Error('LINEAR_CREATE_FIELDS_REQUIRED');
    }

    return {
      provider: 'linear',
      action: 'issue_create',
      issue_id: issueID(input.title),
      issues: [
        {
          issue_id: issueID(input.title),
          title: input.title,
          status: 'Backlog',
          team_id: input.team_id
        }
      ]
    };
  }

  if (!input.issue_id) {
    throw new Error('LINEAR_ISSUE_ID_REQUIRED');
  }

  return {
    provider: 'linear',
    action: 'issue_update',
    issue_id: input.issue_id,
    issues: [
      {
        issue_id: input.issue_id,
        title: input.title ?? 'Updated issue',
        status: input.status ?? 'In Progress',
        team_id: input.team_id ?? 'ENG'
      }
    ]
  };
}
