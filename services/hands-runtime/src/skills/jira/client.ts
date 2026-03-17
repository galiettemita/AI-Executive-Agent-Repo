// Plan §6 steps 12–14 — Real Jira /rest/api/3 with Basic auth
// Replaces: fictional issues

import type { JiraInput, JiraOutput } from './types.js';

interface JiraIssueData {
  key: string;
  fields: {
    summary?: string;
    status?: { name?: string };
    project?: { key?: string };
  };
}

interface JiraSearchResponse {
  issues?: JiraIssueData[];
}

interface JiraCreateResponse {
  key: string;
}

interface JiraTransition {
  id: string;
  name: string;
}

interface JiraTransitionsResponse {
  transitions?: JiraTransition[];
}

export async function runClient(input: JiraInput): Promise<JiraOutput> {
  const baseUrl = process.env.JIRA_BASE_URL;
  const email = process.env.JIRA_EMAIL;
  const apiToken = process.env.JIRA_API_TOKEN;

  if (!baseUrl || !email || !apiToken) {
    throw new Error('jira: JIRA_BASE_URL, JIRA_EMAIL, and JIRA_API_TOKEN must be set');
  }

  // Plan §6 step 12: "Authorization: Basic base64(email:token)"
  const basicAuth = 'Basic ' + Buffer.from(`${email}:${apiToken}`).toString('base64');

  const headers = {
    'Authorization': basicAuth,
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };

  if (input.action === 'issue_list') {
    // Plan §6 step 12: GET /rest/api/3/search?jql=...
    const jql = encodeURIComponent(
      input.project_key
        ? `project = ${input.project_key} ORDER BY created DESC`
        : 'order by created DESC'
    );
    const url = `${baseUrl}/rest/api/3/search?jql=${jql}&maxResults=20&fields=summary,status,project`;

    const response = await fetch(url, {
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`jira: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as JiraSearchResponse;

    return {
      provider: 'jira',
      action: 'issue_list',
      issues: (data.issues ?? []).map((i) => ({
        issue_key: i.key,
        summary: i.fields.summary ?? '',
        status: i.fields.status?.name ?? '',
        project_key: i.fields.project?.key ?? '',
      })),
    };
  }

  if (input.action === 'issue_create') {
    if (!input.project_key || !input.summary) {
      throw new Error('jira: project_key and summary are required for issue_create');
    }

    // Plan §6 step 13: POST /rest/api/3/issue with ADF description
    const response = await fetch(`${baseUrl}/rest/api/3/issue`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        fields: {
          project: { key: input.project_key },
          summary: input.summary,
          description: {
            type: 'doc',
            version: 1,
            content: [
              {
                type: 'paragraph',
                content: [
                  { type: 'text', text: input.description ?? '' },
                ],
              },
            ],
          },
          issuetype: { name: 'Task' },
        },
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`jira: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as JiraCreateResponse;
    return {
      provider: 'jira',
      action: 'issue_create',
      issue_key: data.key,
      issues: [
        {
          issue_key: data.key,
          summary: input.summary,
          status: 'To Do',
          project_key: input.project_key,
        },
      ],
    };
  }

  if (input.action === 'issue_transition') {
    if (!input.issue_key || !input.transition_to) {
      throw new Error('jira: issue_key and transition_to are required for issue_transition');
    }

    // Plan §6 step 14: Step 1 — GET /transitions to find ID by name
    const tUrl = `${baseUrl}/rest/api/3/issue/${input.issue_key}/transitions`;
    const tResponse = await fetch(tUrl, {
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (!tResponse.ok) {
      const text = await tResponse.text();
      throw new Error(`jira: HTTP ${tResponse.status} – ${text.slice(0, 300)}`);
    }

    const tData = (await tResponse.json()) as JiraTransitionsResponse;
    const match = (tData.transitions ?? []).find(
      (t) => t.name.toLowerCase() === input.transition_to!.toLowerCase()
    );

    if (!match) {
      throw new Error(`jira: transition not found: "${input.transition_to}"`);
    }

    // Plan §6 step 14: Step 2 — POST /transitions with {transition:{id}}
    const execResponse = await fetch(tUrl, {
      method: 'POST',
      headers,
      body: JSON.stringify({ transition: { id: match.id } }),
      signal: AbortSignal.timeout(10000),
    });

    if (!execResponse.ok) {
      const text = await execResponse.text();
      throw new Error(`jira: HTTP ${execResponse.status} – ${text.slice(0, 300)}`);
    }

    return {
      provider: 'jira',
      action: 'issue_transition',
      issue_key: input.issue_key,
      issues: [
        {
          issue_key: input.issue_key,
          summary: '',
          status: input.transition_to,
          project_key: input.project_key ?? '',
        },
      ],
    };
  }

  throw new Error(`jira: unknown action ${input.action}`);
}
