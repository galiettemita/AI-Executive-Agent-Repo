// Dispatch Table — binds tool ids to their executors.
//
// The Permission Gate decides WHETHER a tool may run. The dispatch table
// is the thing that actually RUNS it. Phase 3A introduces dispatch for the
// three internal capabilities (audit.write, feedback.write,
// memory_signal.write) — their executors are pure store-write wrappers.
// Phase 3B/3D add external tool executors (gmail.read, sendblue, slack)
// behind real adapters.
//
// Fail-closed invariants (every path returns a DispatchResult; the
// dispatcher never throws unprompted):
//   * unknown tool id          → ok:false code='unknown_tool'
//   * tool exists but no executor registered → ok:false code='no_executor_for_tool'
//   * executor throws          → ok:false code='executor_error', reason carries the message
//
// Phase 3A does NOT couple dispatch to the Permission Gate. The caller is
// responsible for: gate decision → if allowed → dispatch.execute → record
// tool_invocations. Decoupling keeps the gate and dispatch testable in
// isolation and lets a future caller short-circuit dispatch for cached or
// pre-authorized invocations without bypassing the gate's checks.

import { isToolId, type ToolId } from '../core/tool-registry.js';

export interface DispatchContext {
  // The user the executor acts on behalf of. Executors that write to
  // per-user stores read this.
  readonly user_id: string;
  // Caller-supplied dedup id. The dispatcher does NOT enforce uniqueness;
  // it only forwards. The caller correlates it with the tool_invocations
  // record they write.
  readonly invocation_id: string;
}

export type DispatchErrorCode = 'unknown_tool' | 'no_executor_for_tool' | 'executor_error';

export type DispatchResult<TOutput = unknown> =
  | { readonly ok: true; readonly output: TOutput; readonly latency_ms: number }
  | {
      readonly ok: false;
      readonly code: DispatchErrorCode;
      readonly reason: string;
      readonly latency_ms: number;
    };

export type Executor<TArgs = unknown, TOutput = unknown> = (
  args: TArgs,
  context: DispatchContext
) => Promise<TOutput>;

export interface DispatchTable {
  register<TArgs = unknown, TOutput = unknown>(toolId: ToolId, executor: Executor<TArgs, TOutput>): void;
  execute<TOutput = unknown>(
    toolId: string,
    args: unknown,
    context: DispatchContext
  ): Promise<DispatchResult<TOutput>>;
  hasExecutor(toolId: string): boolean;
  registeredToolIds(): readonly ToolId[];
}

export function createDispatchTable(): DispatchTable {
  const executors = new Map<ToolId, Executor>();

  return {
    register(toolId, executor) {
      executors.set(toolId, executor as Executor);
    },

    async execute<TOutput = unknown>(
      toolId: string,
      args: unknown,
      context: DispatchContext
    ): Promise<DispatchResult<TOutput>> {
      const start = Date.now();
      if (!isToolId(toolId)) {
        return Object.freeze({
          ok: false as const,
          code: 'unknown_tool' as const,
          reason: `tool '${toolId}' is not in the v0.1 registry`,
          latency_ms: Date.now() - start
        });
      }
      const executor = executors.get(toolId);
      if (!executor) {
        return Object.freeze({
          ok: false as const,
          code: 'no_executor_for_tool' as const,
          reason: `no executor registered for tool '${toolId}'`,
          latency_ms: Date.now() - start
        });
      }
      try {
        const output = await executor(args, context);
        return Object.freeze({
          ok: true as const,
          output: output as TOutput,
          latency_ms: Date.now() - start
        });
      } catch (err) {
        return Object.freeze({
          ok: false as const,
          code: 'executor_error' as const,
          reason: err instanceof Error ? err.message : String(err),
          latency_ms: Date.now() - start
        });
      }
    },

    hasExecutor(toolId) {
      return isToolId(toolId) && executors.has(toolId);
    },

    registeredToolIds(): readonly ToolId[] {
      return Object.freeze([...executors.keys()]);
    }
  };
}
