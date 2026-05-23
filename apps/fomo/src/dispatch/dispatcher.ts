// Dispatch Table — binds tool ids to their executors.
//
// The Permission Gate decides WHETHER a tool may run. The dispatch table
// is the thing that actually RUNS it. Phase 3A introduces dispatch for the
// three internal capabilities (audit.write, feedback.write,
// memory_signal.write) — their executors are pure store-write wrappers.
// Phase 3B/3D/3E add external tool executors behind real adapters.
//
// Authorization invariant (Phase 3A.1):
//
//   dispatch.execute() is STRUCTURALLY UNREACHABLE without an allowed
//   Permission Gate decision. The signature accepts an AuthorizedToolCall,
//   not a raw tool_id. The only way to construct an AuthorizedToolCall
//   is AuthorizedToolCall.fromDecision(decision), and that factory
//   returns null unless decision.allowed === true && decision.code ===
//   'allowed'. This is the structural guarantee that replaces Phase 3A's
//   "discipline at the call site."
//
//   AuthorizedToolCall uses a private class constructor (TypeScript
//   nominal typing) AND a runtime instanceof check inside execute(). A
//   plain object literal cast to AuthorizedToolCall fails the instanceof
//   check and is denied with code 'unauthorized'.
//
// Fail-closed invariants (every path returns a DispatchResult; the
// dispatcher never throws unprompted):
//   * not an AuthorizedToolCall → ok:false code='unauthorized'
//   * unknown tool id           → ok:false code='unknown_tool'
//   * tool exists but no executor registered → ok:false code='no_executor_for_tool'
//   * executor throws           → ok:false code='executor_error', reason carries the message

import { type PolicyDecision } from '../core/policy-gate.js';
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

/* ====================================================================== *
 * AuthorizedToolCall                                                     *
 *                                                                        *
 * The capability token that proves a Permission Gate decision allowed    *
 * this invocation. Constructor is private — only fromDecision() can      *
 * mint one. Callers cannot fabricate an AuthorizedToolCall through       *
 * normal TypeScript code, and dispatch.execute() rejects forged values   *
 * at runtime.                                                            *
 * ====================================================================== */

export class AuthorizedToolCall {
  readonly tool_id: ToolId;
  readonly user_id: string;
  readonly authorized_at: string;

  // The private constructor is the type-level lock. TypeScript treats
  // it as nominal: a plain object literal { tool_id, user_id, ... } is
  // NOT assignable to AuthorizedToolCall because it lacks the inherited
  // private constructor footprint. Code that tries
  //   const fake: AuthorizedToolCall = { tool_id: 'x', ... }
  // fails to compile.
  private constructor(tool_id: ToolId, user_id: string) {
    this.tool_id = tool_id;
    this.user_id = user_id;
    this.authorized_at = new Date().toISOString();
    Object.freeze(this);
  }

  // The only factory. Returns null unless the decision was explicitly
  // allowed AND the tool id is in the v0.1 registry. The tool-registry
  // check is defense-in-depth — the gate should never return allowed for
  // an unknown tool, but if it ever did, this still refuses.
  static fromDecision(decision: PolicyDecision): AuthorizedToolCall | null {
    if (!decision.allowed) return null;
    if (decision.code !== 'allowed') return null;
    if (!isToolId(decision.tool_id)) return null;
    return new AuthorizedToolCall(decision.tool_id as ToolId, decision.user_id);
  }
}

/* ====================================================================== *
 * Dispatch result + table                                                *
 * ====================================================================== */

export type DispatchErrorCode =
  | 'unauthorized'
  | 'unknown_tool'
  | 'no_executor_for_tool'
  | 'executor_error';

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
  // The signature is the load-bearing safety boundary. Note: AuthorizedToolCall
  // is a class with a private constructor, so callers cannot pass a string
  // tool_id or fabricate an object literal here.
  execute<TOutput = unknown>(
    authorized: AuthorizedToolCall,
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
      authorized: AuthorizedToolCall,
      args: unknown,
      context: DispatchContext
    ): Promise<DispatchResult<TOutput>> {
      const start = Date.now();

      // Runtime defense against forgery: even if a caller used `as unknown
      // as AuthorizedToolCall` to bypass the TypeScript type system,
      // instanceof catches a plain object that did not come from
      // AuthorizedToolCall.fromDecision().
      if (!(authorized instanceof AuthorizedToolCall)) {
        return Object.freeze({
          ok: false as const,
          code: 'unauthorized' as const,
          reason: 'dispatch.execute requires an AuthorizedToolCall from AuthorizedToolCall.fromDecision()',
          latency_ms: Date.now() - start
        });
      }

      const toolId = authorized.tool_id;

      if (!isToolId(toolId)) {
        // Defense-in-depth — fromDecision() already filters unknown tools,
        // but if some path slipped through, we still refuse.
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
