// Model Router — the substrate every future model call goes through.
//
// FOMO_PLAN §9.8. Responsibilities:
//   * register one backend per capability tag (v0.1: 'classification' only)
//   * call the backend with a bounded timeout
//   * validate the backend's text output via an injected validator function
//     (router stays library-agnostic — caller brings Zod / JSON Schema / etc.)
//   * fail closed on every error: unknown capability, no backend, backend
//     error, timeout, schema-invalid output
//   * write a CostRecord for every backend call that produced tokens,
//     including schema-invalid calls (wasted spend is real spend)
//
// Phase 2D ships substrate + a deterministic MockModelBackend. No real
// OpenAI/Anthropic adapters yet — those land with the first real prompt
// in Phase 3. CI never makes a network call.

import {
  type CapabilityTag,
  type CostStore,
  computeEstimatedCost,
  isCapabilityTag
} from './cost-tracking.js';

export interface BackendResult {
  // Raw text the model returned. The router does not parse this — it hands
  // the text to the caller's validator function which decides shape.
  text: string;
  input_tokens: number;
  output_tokens: number;
  // The backend self-identifies. Cost record carries this as model_name.
  model_name: string;
  // Wall-clock the backend took, excluding router overhead.
  latency_ms: number;
}

export interface ModelBackend {
  name(): string;
  call(request: { prompt: string; timeout_ms: number }): Promise<BackendResult>;
}

// Validator function the caller supplies. Returns either a typed value or a
// reason string. Router treats any { ok: false } as schema_invalid.
export type ModelOutputValidator<TOutput> = (
  text: string
) => { ok: true; value: TOutput } | { ok: false; reason: string };

export interface ModelRouteRequest<TOutput = unknown> {
  capability: CapabilityTag;
  prompt: string;
  // Records to cost_records.prompt_version so a future bake-off can attribute
  // wins/regressions to specific prompt versions.
  prompt_version: string;
  user_id: string;
  validate: ModelOutputValidator<TOutput>;
  // Per-call timeout override. Defaults to the router's defaultTimeoutMs.
  timeout_ms?: number;
}

export interface ModelRouteSuccess<TOutput> {
  readonly ok: true;
  readonly output: TOutput;
  readonly model_name: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
}

export type ModelRouteErrorCode =
  | 'unknown_capability'
  | 'no_backend_for_capability'
  | 'backend_error'
  | 'timeout'
  | 'schema_invalid';

export interface ModelRouteError {
  readonly ok: false;
  readonly code: ModelRouteErrorCode;
  readonly reason: string;
  // Set only for errors that did consume tokens (schema_invalid). null for
  // errors that never reached or never completed the backend call.
  readonly model_name: string | null;
}

export type ModelRouteResult<TOutput> = ModelRouteSuccess<TOutput> | ModelRouteError;

export interface ModelRouter {
  registerBackend(capability: CapabilityTag, backend: ModelBackend): void;
  route<TOutput>(req: ModelRouteRequest<TOutput>): Promise<ModelRouteResult<TOutput>>;
}

export interface ModelRouterConfig {
  costStore: CostStore;
  defaultTimeoutMs?: number;
}

function withTimeout<T>(p: Promise<T>, ms: number): Promise<T> {
  return new Promise((resolve, reject) => {
    const handle = setTimeout(() => reject(new Error('__router_timeout__')), ms);
    p.then(
      (v) => {
        clearTimeout(handle);
        resolve(v);
      },
      (e: unknown) => {
        clearTimeout(handle);
        reject(e instanceof Error ? e : new Error(String(e)));
      }
    );
  });
}

export function createModelRouter(config: ModelRouterConfig): ModelRouter {
  const backends = new Map<CapabilityTag, ModelBackend>();
  const defaultTimeoutMs = config.defaultTimeoutMs ?? 30_000;

  return {
    registerBackend(capability, backend) {
      if (!isCapabilityTag(capability)) {
        throw new Error(`Model Router: unknown capability tag '${capability}'`);
      }
      backends.set(capability, backend);
    },

    async route<TOutput>(req: ModelRouteRequest<TOutput>): Promise<ModelRouteResult<TOutput>> {
      if (!isCapabilityTag(req.capability)) {
        return Object.freeze({
          ok: false as const,
          code: 'unknown_capability' as const,
          reason: `capability tag '${req.capability}' is not in the v0.1 set`,
          model_name: null
        });
      }
      const backend = backends.get(req.capability);
      if (!backend) {
        return Object.freeze({
          ok: false as const,
          code: 'no_backend_for_capability' as const,
          reason: `no backend registered for capability '${req.capability}'`,
          model_name: null
        });
      }

      const timeout_ms = req.timeout_ms ?? defaultTimeoutMs;
      let result: BackendResult;
      try {
        result = await withTimeout(backend.call({ prompt: req.prompt, timeout_ms }), timeout_ms);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        if (msg === '__router_timeout__') {
          return Object.freeze({
            ok: false as const,
            code: 'timeout' as const,
            reason: `backend ${backend.name()} exceeded timeout of ${timeout_ms}ms`,
            model_name: backend.name()
          });
        }
        return Object.freeze({
          ok: false as const,
          code: 'backend_error' as const,
          reason: `backend ${backend.name()} threw: ${msg}`,
          model_name: backend.name()
        });
      }

      const estimated_cost_usd = computeEstimatedCost(
        result.model_name,
        result.input_tokens,
        result.output_tokens
      );

      const validation = req.validate(result.text);

      // Always write a cost record once the backend has consumed tokens —
      // including schema-invalid calls. Wasted spend is real spend and must
      // be visible in the cost ledger.
      await config.costStore.write({
        user_id: req.user_id,
        capability: req.capability,
        model_name: result.model_name,
        prompt_version: req.prompt_version,
        latency_ms: result.latency_ms,
        input_tokens: result.input_tokens,
        output_tokens: result.output_tokens,
        estimated_cost_usd,
        schema_valid: validation.ok
      });

      if (!validation.ok) {
        return Object.freeze({
          ok: false as const,
          code: 'schema_invalid' as const,
          reason: `backend ${result.model_name} output failed validation: ${validation.reason}`,
          model_name: result.model_name
        });
      }

      return Object.freeze({
        ok: true as const,
        output: validation.value,
        model_name: result.model_name,
        latency_ms: result.latency_ms,
        input_tokens: result.input_tokens,
        output_tokens: result.output_tokens,
        estimated_cost_usd
      });
    }
  };
}
