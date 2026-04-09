import type { Channel, GatewayConfig } from './types.js';

interface StartMessageWorkflowInput {
  messageId: string;
  userId: string;
  channel: Channel;
  channelMessageId: string;
  sessionId: string;
  messageText?: string;
  userProfileHash: string;
}

interface RuntimeResponsePayload {
  run_id?: unknown;
}

export interface StartMessageWorkflowResult {
  delegated: boolean;
  runId?: string;
  warning?: string;
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function runtimeUrl(config: Pick<GatewayConfig, 'temporalWorkerBaseUrl'>): string | undefined {
  const base = config.temporalWorkerBaseUrl?.trim();
  return base && base.length > 0 ? `${base.replace(/\/+$/, '')}/api/v1/temporal-worker/workflows/message-processing` : undefined;
}

export async function startMessageWorkflow(
  input: StartMessageWorkflowInput,
  config: Pick<GatewayConfig, 'temporalWorkerBaseUrl' | 'temporalWorkerTimeoutMs'>,
  fetchImpl: typeof fetch = fetch
): Promise<StartMessageWorkflowResult> {
  const url = runtimeUrl(config);
  if (!url) {
    return { delegated: false };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(url, {
      method: 'POST',
      signal: controller.signal,
      headers: {
        'content-type': 'application/json'
      },
      body: JSON.stringify({
        message_id: input.messageId,
        user_id: input.userId,
        channel: input.channel,
        session_id: input.sessionId,
        channel_message_id: input.channelMessageId,
        message_text: input.messageText,
        user_profile_hash: input.userProfileHash,
        pause_after_state: 'RECEIVED'
      })
    });

    if (!response.ok) {
      return {
        delegated: false,
        warning: `temporal_worker_start_failed_status_${response.status}`
      };
    }

    const payload = (await response.json()) as RuntimeResponsePayload;
    return {
      delegated: true,
      runId: asString(payload.run_id) ?? input.messageId
    };
  } catch (error) {
    return {
      delegated: false,
      warning:
        error instanceof Error && error.name === 'AbortError'
          ? 'temporal_worker_start_timeout'
          : 'temporal_worker_start_unavailable'
    };
  } finally {
    clearTimeout(timeout);
  }
}
