import type { ExecutionLifecycleStatus, ExecutionRecord } from './execution-store.js';

interface RuntimeResponsePayload {
  step_id?: unknown;
}

export interface EdgeWorkflowReportResult {
  delegated: boolean;
  warning?: string;
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function baseUrl(config: { temporalWorkerBaseUrl?: string }): string | undefined {
  const value = config.temporalWorkerBaseUrl?.trim();
  return value && value.length > 0 ? value.replace(/\/+$/, '') : undefined;
}

function isTerminalStatus(status: ExecutionLifecycleStatus): boolean {
  return ['SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT', 'NEEDS_CONSENT', 'NOT_EXECUTED', 'SIMULATED', 'REJECTED'].includes(status);
}

function mapStatus(status: ExecutionLifecycleStatus): 'COMPLETED' | 'FAILED' {
  return status === 'SUCCESS' || status === 'PARTIAL' ? 'COMPLETED' : 'FAILED';
}

function artifactPayload(record: ExecutionRecord) {
  const inlineData = record.result?.data
    ?? (record.lastError ? { error: record.lastError } : undefined);
  if (inlineData === undefined) {
    return undefined;
  }
  return [
    {
      artifact_id: `${record.stepId ?? record.requestId}:edge-result`,
      type: record.result?.error || record.lastError ? 'error' : 'skill_result',
      inline_data: inlineData
    }
  ];
}

export async function reportExecutionLifecycle(
  record: ExecutionRecord | undefined | null,
  config: { temporalWorkerBaseUrl?: string; temporalWorkerTimeoutMs: number },
  fetchImpl: typeof fetch = fetch
): Promise<EdgeWorkflowReportResult> {
  const runtimeBaseUrl = baseUrl(config);
  if (!record || !runtimeBaseUrl || !record.runId || !record.stepId || !isTerminalStatus(record.status)) {
    return { delegated: false };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(
      `${runtimeBaseUrl}/api/v1/temporal-worker/runs/${encodeURIComponent(record.runId)}/planner-steps/${encodeURIComponent(record.stepId)}/transition`,
      {
        method: 'POST',
        signal: controller.signal,
        headers: {
          'content-type': 'application/json'
        },
        body: JSON.stringify({
          status: mapStatus(record.status),
          artifacts: artifactPayload(record),
          error: record.result?.error ?? record.lastError,
          metadata: {
            request_id: record.requestId,
            task_id: record.taskId,
            attempt: record.attempt,
            skill_id: record.skillId,
            lifecycle_status: record.status,
            latency_ms: record.result?.latencyMs
          }
        })
      }
    );

    if (!response.ok) {
      return {
        delegated: false,
        warning: `temporal_worker_result_report_failed_status_${response.status}`
      };
    }

    const payload = (await response.json()) as RuntimeResponsePayload;
    return {
      delegated: Boolean(asString(payload.step_id))
    };
  } catch (error) {
    return {
      delegated: false,
      warning:
        error instanceof Error && error.name === 'AbortError'
          ? 'temporal_worker_result_report_timeout'
          : 'temporal_worker_result_report_unavailable'
    };
  } finally {
    clearTimeout(timeout);
  }
}
