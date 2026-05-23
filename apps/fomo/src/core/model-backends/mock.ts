// MockModelBackend — deterministic backend for tests and dev.
//
// Configure with a model name + a map from canonical prompt → canned response.
// The router calls .call({prompt, timeout_ms}); the mock looks the prompt up
// (string equality after trim/collapse) and returns the response. Optional
// per-response latency_ms simulates slow backends; if latency exceeds the
// requested timeout, the mock throws a __mock_timeout__ marker that the
// router converts into its standard timeout deny.
//
// No network. No SDK. Phase 2D's tests use only this backend; real
// OpenAI/Anthropic adapters land in Phase 3 with the first real prompt.

import type { BackendResult, ModelBackend } from '../model-router.js';

export interface MockResponse {
  text: string;
  input_tokens: number;
  output_tokens: number;
  // Simulated backend latency. The mock awaits this before resolving.
  latency_ms?: number;
}

export interface MockModelBackendConfig {
  model_name: string;
  // Exact-match map from prompt to canned response. Keys are normalized via
  // normalizePrompt (whitespace-collapsed + trimmed).
  responses?: Record<string, MockResponse>;
  // Returned when no response key matches the incoming prompt.
  default?: MockResponse;
}

export function normalizePrompt(prompt: string): string {
  return prompt.replace(/\s+/g, ' ').trim();
}

export class MockModelBackend implements ModelBackend {
  private readonly model_name: string;
  private readonly responses: Map<string, MockResponse>;
  private readonly defaultResponse: MockResponse | undefined;

  constructor(config: MockModelBackendConfig) {
    this.model_name = config.model_name;
    this.responses = new Map();
    for (const [k, v] of Object.entries(config.responses ?? {})) {
      this.responses.set(normalizePrompt(k), v);
    }
    this.defaultResponse = config.default;
  }

  name(): string {
    return this.model_name;
  }

  async call(request: { prompt: string; timeout_ms: number }): Promise<BackendResult> {
    const key = normalizePrompt(request.prompt);
    const response = this.responses.get(key) ?? this.defaultResponse;
    if (!response) {
      throw new Error(
        `MockModelBackend(${this.model_name}): no canned response for prompt and no default configured`
      );
    }
    const latency_ms = response.latency_ms ?? 0;
    if (latency_ms > request.timeout_ms) {
      // Simulate a slow backend that the router will time out. Wait the
      // timeout window so the router's withTimeout race resolves on its
      // side; then throw so a hypothetical caller without a router timeout
      // also sees a clear failure.
      await new Promise((resolve) => setTimeout(resolve, request.timeout_ms + 1));
      throw new Error(
        `MockModelBackend(${this.model_name}): simulated latency ${latency_ms}ms exceeds timeout ${request.timeout_ms}ms`
      );
    }
    if (latency_ms > 0) {
      await new Promise((resolve) => setTimeout(resolve, latency_ms));
    }
    return {
      text: response.text,
      input_tokens: response.input_tokens,
      output_tokens: response.output_tokens,
      model_name: this.model_name,
      latency_ms
    };
  }
}
