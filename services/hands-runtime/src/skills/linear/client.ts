// Plan §6 steps 15–16 — Real Linear GraphQL API
// CRITICAL: Authorization header uses raw API key, not a token prefix (plan §6 step 15)
// Replaces: fictional issues

import type { LinearInput, LinearOutput } from './types.js';

interface LinearNode {
  id: string;
  title: string;
  state?: { name?: string };
  team?: { id?: string };
}

interface LinearIssueListResponse {
  data?: {
    team?: {
      issues?: {
        nodes?: LinearNode[];
      };
    };
  };
  errors?: Array<{ message: string }>;
}

interface LinearCreateResponse {
  data?: {
    issueCreate?: {
      success?: boolean;
      issue?: { id?: string };
    };
  };
  errors?: Array<{ message: string }>;
}

// Private helper: execute a GraphQL operation against Linear's single endpoint
async function linearRequest<T extends { errors?: Array<{ message: string }> }>(
  apiKey: string,
  query: string,
  variables: Record<string, unknown>
): Promise<T> {
  const response = await fetch('https://api.linear.app/graphql', {
    method: 'POST',
    headers: {
      'Authorization': apiKey,          // plan: raw key, no prefix
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query, variables }),
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`linear: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as T;
  const firstError = data.errors?.[0];
  if (firstError) {
    throw new Error(`linear: ${firstError.message}`);
  }
  return data;
}

export async function runClient(input: LinearInput): Promise<LinearOutput> {
  const key = process.env.LINEAR_API_KEY;
  if (!key) throw new Error('linear: LINEAR_API_KEY not set');

  if (input.action === 'issue_list') {
    // Plan §6 step 15: query Issues($teamId)
    const data = await linearRequest<LinearIssueListResponse>(
      key,
      `query Issues($teamId: String!) {
        team(id: $teamId) {
          issues {
            nodes {
              id
              title
              state { name }
              team { id }
            }
          }
        }
      }`,
      { teamId: input.team_id ?? '' }
    );

    const nodes = data.data?.team?.issues?.nodes ?? [];
    return {
      provider: 'linear',
      action: 'issue_list',
      issues: nodes.map((n) => ({
        issue_id: n.id,
        title: n.title,
        status: n.state?.name ?? '',
        team_id: n.team?.id ?? input.team_id ?? '',
      })),
    };
  }

  if (input.action === 'issue_create') {
    if (!input.title || !input.team_id) {
      throw new Error('linear: title and team_id are required for issue_create');
    }

    // Plan §6 step 16: mutation CreateIssue($title,$teamId,$description)
    const data = await linearRequest<LinearCreateResponse>(
      key,
      `mutation CreateIssue($title: String!, $teamId: String!, $description: String) {
        issueCreate(input: { title: $title, teamId: $teamId, description: $description }) {
          success
          issue { id }
        }
      }`,
      {
        title: input.title,
        teamId: input.team_id,
        description: input.description ?? '',
      }
    );

    const id = data.data?.issueCreate?.issue?.id;
    if (!id) throw new Error('linear: issueCreate returned no issue id');

    return {
      provider: 'linear',
      action: 'issue_create',
      issue_id: id,
      issues: [
        {
          issue_id: id,
          title: input.title,
          status: 'Backlog',
          team_id: input.team_id,
        },
      ],
    };
  }

  throw new Error(`linear: unknown action ${input.action}`);
}
