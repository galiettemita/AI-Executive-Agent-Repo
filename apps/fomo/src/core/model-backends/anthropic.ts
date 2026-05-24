// Anthropic Messages API backend — Phase 3C.1.
//
// First real ModelBackend in the project. Pairs with MockModelBackend
// (Phase 2D) — same `ModelBackend` interface, plug-replaceable through
// the model router. The router-and-validator surface is unchanged.
//
// Design choices (mirror the other adapters in this codebase):
//   * Direct fetch, no `@anthropic-ai/sdk`. Smaller lockfile, auditable
//     call surface — same posture as GmailClient + oauth-exchange.
//   * Injectable FetchLike — tests inject mocks; CI never hits the real
//     API. The Phase 3C.1 unit tests fully exercise success + every
//     fail-closed path without network.
//   * Caller-supplied API key + model id at construction time. The
//     backend self-identifies via .name() so cost_records record the
//     exact model that ran.
//   * No retry / backoff at this layer. The router's per-call timeout
//     is the only timing guarantee; 429/5xx surface as backend_error
//     and the router records the failure. A retry layer can wrap this
//     backend later without changing the interface.
//
// Pricing: see core/cost-tracking.ts MODEL_PRICING — Anthropic Haiku +
// Sonnet entries are added in 3C.1 so computeEstimatedCost returns
// non-zero for these models.

import { type BackendResult, type ModelBackend } from '../model-router.js';

export type FetchLike = typeof fetch;

// Two models registered for the v0.1 bake-off planned in 3C.2. Adding
// new model ids here requires a corresponding MODEL_PRICING entry in
// core/cost-tracking.ts; computeEstimatedCost returns 0 otherwise.
export type AnthropicModelId =
  | 'claude-haiku-4-5-20251001'
  | 'claude-sonnet-4-6';

export const ANTHROPIC_API_BASE = 'https://api.anthropic.com/v1';
export const ANTHROPIC_VERSION_HEADER = '2023-06-01';

export class AnthropicAuthError extends Error {
  readonly httpStatus: number;
  constructor(httpStatus: number, reason: string) {
    super(`Anthropic auth error (${httpStatus}): ${reason}`);
    this.name = 'AnthropicAuthError';
    this.httpStatus = httpStatus;
  }
}

export class AnthropicApiError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  readonly retryable: boolean;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(`Anthropic API error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'AnthropicApiError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
    // Same classification as GmailClient: 5xx + 429 + 529 (Anthropic
    // "overloaded") are retryable, everything else is not.
    this.retryable = httpStatus >= 500 || httpStatus === 429 || httpStatus === 529;
  }
}

export interface AnthropicBackendConfig {
  readonly apiKey: string;
  readonly model: AnthropicModelId;
  // Max output tokens per call. Defaults to 1024 — enough for the
  // ranker JSON shape; raise if a future capability needs longer
  // responses.
  readonly maxOutputTokens?: number;
  // Sampling temperature. Defaults to 0 for classifier-style use
  // (deterministic JSON output is the goal).
  readonly temperature?: number;
  // Inject for tests; defaults to global fetch.
  readonly fetchImpl?: FetchLike;
}

interface AnthropicMessagesResponse {
  readonly id: string;
  readonly type: 'message';
  readonly role: 'assistant';
  readonly model: string;
  readonly content: ReadonlyArray<{ readonly type: string; readonly text?: string }>;
  readonly stop_reason?: string;
  readonly usage: {
    readonly input_tokens: number;
    readonly output_tokens: number;
  };
}

export class AnthropicBackend implements ModelBackend {
  private readonly apiKey: string;
  private readonly model: AnthropicModelId;
  private readonly maxOutputTokens: number;
  private readonly temperature: number;
  private readonly fetchImpl: FetchLike;

  constructor(config: AnthropicBackendConfig) {
    if (!config.apiKey || config.apiKey.length === 0) {
      throw new Error('AnthropicBackend: apiKey is required');
    }
    this.apiKey = config.apiKey;
    this.model = config.model;
    this.maxOutputTokens = config.maxOutputTokens ?? 1024;
    this.temperature = config.temperature ?? 0;
    this.fetchImpl = config.fetchImpl ?? fetch;
  }

  name(): string {
    return this.model;
  }

  async call(request: { prompt: string; timeout_ms: number }): Promise<BackendResult> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), request.timeout_ms);

    const body = {
      model: this.model,
      max_tokens: this.maxOutputTokens,
      temperature: this.temperature,
      messages: [{ role: 'user', content: request.prompt }]
    };

    const startedAt = Date.now();
    let res: Response;
    try {
      res = await this.fetchImpl(`${ANTHROPIC_API_BASE}/messages`, {
        method: 'POST',
        headers: {
          'x-api-key': this.apiKey,
          'anthropic-version': ANTHROPIC_VERSION_HEADER,
          'content-type': 'application/json',
          accept: 'application/json'
        },
        body: JSON.stringify(body),
        signal: controller.signal
      });
    } catch (err) {
      clearTimeout(timer);
      throw new AnthropicApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }
    clearTimeout(timer);

    if (res.status === 401 || res.status === 403) {
      throw new AnthropicAuthError(res.status, 'API key rejected or unauthorized');
    }

    let parsed: unknown;
    try {
      parsed = await res.json();
    } catch {
      if (!res.ok) {
        throw new AnthropicApiError(res.status, undefined, 'non-JSON response');
      }
      throw new AnthropicApiError(res.status, undefined, 'response body not parseable as JSON');
    }

    if (!res.ok) {
      const errObj = (parsed as { error?: { type?: string; message?: string } }).error;
      throw new AnthropicApiError(
        res.status,
        errObj?.type,
        errObj?.message ?? `HTTP ${res.status}`
      );
    }

    const data = parsed as AnthropicMessagesResponse;
    // Concatenate all text-typed content blocks. Anthropic returns an
    // array; for classifier prompts there's typically one text block.
    const text = (data.content ?? [])
      .filter((c) => c.type === 'text' && typeof c.text === 'string')
      .map((c) => c.text as string)
      .join('');

    return Object.freeze({
      text,
      input_tokens: data.usage?.input_tokens ?? 0,
      output_tokens: data.usage?.output_tokens ?? 0,
      model_name: this.model,
      latency_ms: Date.now() - startedAt
    });
  }
}
