// OpenAI Chat Completions backend — Phase 3C.2.
//
// Founder-directed initial Brevio ranker provider. Sits next to
// AnthropicBackend (Phase 3C.1, dormant in main) under the same
// ModelBackend interface. Same design posture as AnthropicBackend and
// GmailClient:
//   * Direct fetch, no @openai/openai SDK. Auditable call surface,
//     small lockfile.
//   * Injectable FetchLike — every unit test injects a mock; CI never
//     hits api.openai.com.
//   * Caller-supplied API key + model id at construction time. Backend
//     self-identifies via .name() so cost_records carry the exact model.
//   * No retry/backoff at this layer. 401/403 → OpenAIAuthError,
//     429/5xx → OpenAIApiError(retryable=true). A future retry layer
//     can wrap this backend without changing the interface.
//
// Structured output: this backend takes an optional `responseFormat`
// at construction time and forwards it verbatim as
// chat.completions.response_format. For the FOMO ranker, the smoke
// eval (scripts/smoke-eval-3c2.ts) constructs the backend with the
// ranker JSON Schema so OpenAI enforces the shape server-side. The
// existing ranker validator still runs as defense-in-depth.

import { type BackendResult, type ModelBackend } from '../model-router.js';

export type FetchLike = typeof fetch;

export const OPENAI_API_BASE = 'https://api.openai.com/v1';

export class OpenAIAuthError extends Error {
  readonly httpStatus: number;
  constructor(httpStatus: number, reason: string) {
    super(`OpenAI auth error (${httpStatus}): ${reason}`);
    this.name = 'OpenAIAuthError';
    this.httpStatus = httpStatus;
  }
}

export class OpenAIApiError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  readonly retryable: boolean;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(`OpenAI API error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`);
    this.name = 'OpenAIApiError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
    // 5xx and 429 are retryable. 408 (request timeout) also retryable.
    this.retryable = httpStatus >= 500 || httpStatus === 429 || httpStatus === 408;
  }
}

// response_format shapes the OpenAI Chat Completions API accepts.
// Kept narrow on purpose — we don't want callers passing arbitrary
// payloads that aren't valid OpenAI request bodies.
export type OpenAIResponseFormat =
  | { readonly type: 'text' }
  | { readonly type: 'json_object' }
  | {
      readonly type: 'json_schema';
      readonly json_schema: {
        readonly name: string;
        readonly strict: boolean;
        readonly schema: Readonly<Record<string, unknown>>;
      };
    };

export interface OpenAIBackendConfig {
  readonly apiKey: string;
  // Caller-controlled. Phase 3C.2 founder directive: 'gpt-5-mini' as
  // primary. The backend does not validate the id; if OpenAI doesn't
  // recognize it, the call surfaces as OpenAIApiError(400, model_not_found).
  readonly model: string;
  // Sent as max_completion_tokens (newer field). Defaults to 1024 — enough
  // for the ranker JSON; raise for capabilities with longer outputs.
  readonly maxCompletionTokens?: number;
  // 0 for classifier-style use (deterministic).
  readonly temperature?: number;
  // Optional structured-output enforcement. The ranker smoke eval passes
  // {type:'json_schema', json_schema:{name,strict:true,schema}}. When
  // omitted, the model returns free text and the caller's validator is
  // the only guard.
  readonly responseFormat?: OpenAIResponseFormat;
  // Optional org id, sent as OpenAI-Organization header. Most callers
  // can omit; only needed if your API key spans multiple orgs.
  readonly organizationId?: string;
  // Inject for tests; defaults to global fetch.
  readonly fetchImpl?: FetchLike;
}

interface OpenAIChatCompletion {
  readonly id: string;
  readonly object: 'chat.completion';
  readonly model: string;
  readonly choices: ReadonlyArray<{
    readonly index: number;
    readonly message: {
      readonly role: 'assistant';
      readonly content: string | null;
      readonly refusal?: string | null;
    };
    readonly finish_reason: string;
  }>;
  readonly usage?: {
    readonly prompt_tokens: number;
    readonly completion_tokens: number;
    readonly total_tokens: number;
  };
}

export class OpenAIBackend implements ModelBackend {
  private readonly apiKey: string;
  private readonly model: string;
  private readonly maxCompletionTokens: number;
  private readonly temperature: number;
  private readonly responseFormat: OpenAIResponseFormat | undefined;
  private readonly organizationId: string | undefined;
  private readonly fetchImpl: FetchLike;

  constructor(config: OpenAIBackendConfig) {
    if (!config.apiKey || config.apiKey.length === 0) {
      throw new Error('OpenAIBackend: apiKey is required');
    }
    if (!config.model || config.model.length === 0) {
      throw new Error('OpenAIBackend: model is required');
    }
    this.apiKey = config.apiKey;
    this.model = config.model;
    this.maxCompletionTokens = config.maxCompletionTokens ?? 1024;
    this.temperature = config.temperature ?? 0;
    this.responseFormat = config.responseFormat;
    this.organizationId = config.organizationId;
    this.fetchImpl = config.fetchImpl ?? fetch;
  }

  name(): string {
    return this.model;
  }

  async call(request: { prompt: string; timeout_ms: number }): Promise<BackendResult> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), request.timeout_ms);

    const body: Record<string, unknown> = {
      model: this.model,
      messages: [{ role: 'user', content: request.prompt }],
      temperature: this.temperature,
      max_completion_tokens: this.maxCompletionTokens
    };
    if (this.responseFormat) {
      body.response_format = this.responseFormat;
    }

    const headers: Record<string, string> = {
      authorization: `Bearer ${this.apiKey}`,
      'content-type': 'application/json',
      accept: 'application/json'
    };
    if (this.organizationId) {
      headers['openai-organization'] = this.organizationId;
    }

    const startedAt = Date.now();
    let res: Response;
    try {
      res = await this.fetchImpl(`${OPENAI_API_BASE}/chat/completions`, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: controller.signal
      });
    } catch (err) {
      clearTimeout(timer);
      throw new OpenAIApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }
    clearTimeout(timer);

    if (res.status === 401 || res.status === 403) {
      throw new OpenAIAuthError(res.status, 'API key rejected or unauthorized');
    }

    let parsed: unknown;
    try {
      parsed = await res.json();
    } catch {
      if (!res.ok) {
        throw new OpenAIApiError(res.status, undefined, 'non-JSON response');
      }
      throw new OpenAIApiError(res.status, undefined, 'response body not parseable as JSON');
    }

    if (!res.ok) {
      const errObj = (parsed as { error?: { type?: string; code?: string; message?: string } }).error;
      throw new OpenAIApiError(
        res.status,
        errObj?.code ?? errObj?.type,
        errObj?.message ?? `HTTP ${res.status}`
      );
    }

    const data = parsed as OpenAIChatCompletion;
    const choice = data.choices?.[0];
    if (!choice) {
      throw new OpenAIApiError(res.status, 'no_choices', 'OpenAI response had no choices');
    }
    // Surface model refusal as an explicit error. Strict json_schema
    // mode can produce a refusal instead of content when the model
    // can't comply; treating refusal as success would hand the ranker
    // empty text and confuse the validator.
    if (choice.message.refusal) {
      throw new OpenAIApiError(res.status, 'model_refusal', choice.message.refusal);
    }
    const text = choice.message.content ?? '';

    return Object.freeze({
      text,
      input_tokens: data.usage?.prompt_tokens ?? 0,
      output_tokens: data.usage?.completion_tokens ?? 0,
      model_name: this.model,
      latency_ms: Date.now() - startedAt
    });
  }
}
