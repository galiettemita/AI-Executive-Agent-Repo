// Plan §6 steps 22–23 — Real Microsoft Graph /me/messages + /me/sendMail
// OAuth: ctx.token||MICROSOFT_TOKEN per plan §6 step 22 and §4 architecture
// IMPORTANT: sendMail returns HTTP 202, not 200

import type { OutlookInput, OutlookOutput } from './types.js';

const GRAPH_BASE = 'https://graph.microsoft.com/v1.0';

interface GraphMessage {
  id: string;
  from?: { emailAddress?: { address?: string } };
  subject?: string;
}

interface GraphMessagesResponse {
  value?: GraphMessage[];
}

interface GraphEvent {
  id: string;
  subject?: string;
  start?: { dateTime?: string };
}

interface GraphEventsResponse {
  value?: GraphEvent[];
}

// ctx carries the per-user OAuth token injected by Go credential_resolver (plan §4)
export async function runClient(
  input: OutlookInput,
  ctx?: { token?: string }
): Promise<OutlookOutput> {
  const token = ctx?.token || process.env.MICROSOFT_TOKEN;
  if (!token) throw new Error('outlook: MICROSOFT_TOKEN not set');

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };

  if (input.action === 'inbox_list') {
    // Plan §6 step 22: GET /me/messages?$top=20&$orderby=receivedDateTime desc
    const response = await fetch(
      `${GRAPH_BASE}/me/messages?$top=20&$orderby=receivedDateTime desc`,
      { headers, signal: AbortSignal.timeout(10000) }
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`outlook: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as GraphMessagesResponse;

    return {
      provider: 'outlook',
      action: 'inbox_list',
      mails: (data.value ?? []).map((m) => ({
        message_id: m.id,
        from: m.from?.emailAddress?.address ?? '',
        subject: m.subject ?? '',
      })),
    };
  }

  if (input.action === 'calendar_list') {
    const response = await fetch(
      `${GRAPH_BASE}/me/events?$top=20&$orderby=start/dateTime`,
      { headers, signal: AbortSignal.timeout(10000) }
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`outlook: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as GraphEventsResponse;

    return {
      provider: 'outlook',
      action: 'calendar_list',
      events: (data.value ?? []).map((e) => ({
        event_id: e.id,
        subject: e.subject ?? '',
        start_time: e.start?.dateTime ?? '',
      })),
    };
  }

  if (input.action === 'send') {
    // Plan §6 step 23: confirmation gate
    if (!input.confirmed) {
      return {
        provider: 'outlook',
        action: 'send',
        confirmation_required: true,
      };
    }

    if (!input.to || !input.subject || !input.body) {
      throw new Error('outlook: to, subject, and body are required for send');
    }

    // Plan §6 step 23: POST /me/sendMail with exact message structure
    const response = await fetch(`${GRAPH_BASE}/me/sendMail`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        message: {
          subject: input.subject,
          body: {
            contentType: 'Text',
            content: input.body,
          },
          toRecipients: input.to.map((addr) => ({
            emailAddress: { address: addr },
          })),
        },
      }),
      signal: AbortSignal.timeout(10000),
    });

    // Graph sendMail returns 202 Accepted (no body)
    if (response.status !== 202) {
      const text = await response.text().catch(() => '');
      throw new Error(
        `outlook: sendMail failed – expected HTTP 202, got ${response.status} ${text.slice(0, 200)}`
      );
    }

    return {
      provider: 'outlook',
      action: 'send',
      confirmation_required: false,
    };
  }

  throw new Error(`outlook: unknown action ${input.action}`);
}
