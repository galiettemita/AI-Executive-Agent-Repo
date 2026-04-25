import type { SkillResult } from '@brevio/shared';

interface RuntimeResponsePayload {
  step_id?: unknown;
}

export interface HandsWorkflowReportResult {
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

function artifactPayload(result: SkillResult) {
  const inlineData = result.error
    ? { error: result.error, data: result.data }
    : result.data;
  if (inlineData === undefined) {
    return undefined;
  }
  return [
    {
      artifact_id: `${result.step_id ?? result.request_id ?? result.skill_id}:result`,
      type: result.error ? 'error' : 'skill_result',
      inline_data: inlineData
    }
  ];
}

function mapStatus(result: SkillResult): 'COMPLETED' | 'FAILED' {
  return result.status === 'SUCCESS' || result.status === 'PARTIAL' ? 'COMPLETED' : 'FAILED';
}

export async function reportExecutionResult(
  result: SkillResult,
  config: { temporalWorkerBaseUrl?: string; temporalWorkerTimeoutMs: number },
  fetchImpl: typeof fetch = fetch
): Promise<HandsWorkflowReportResult> {
  const runtimeBaseUrl = baseUrl(config);
  if (!runtimeBaseUrl || !result.run_id || !result.step_id) {
    return { delegated: false };
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.temporalWorkerTimeoutMs);

  try {
    const response = await fetchImpl(
      `${runtimeBaseUrl}/api/v1/temporal-worker/runs/${encodeURIComponent(result.run_id)}/planner-steps/${encodeURIComponent(result.step_id)}/transition`,
      {
        method: 'POST',
        signal: controller.signal,
        headers: {
          'content-type': 'application/json'
        },
        body: JSON.stringify({
          status: mapStatus(result),
          artifacts: artifactPayload(result),
          error: result.error,
          metadata: {
            request_id: result.request_id,
            attempt: result.attempt,
            skill_id: result.skill_id,
            result_status: result.status,
            latency_ms: result.latency_ms,
            tokens_used: result.tokens_used,
            cost_cents: result.cost_cents
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
