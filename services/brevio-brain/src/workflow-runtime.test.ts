import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { annotateRunVerification, registerExecutionPlan, syncProcessRunState } from './workflow-runtime.js';

describe('brain workflow runtime sync', () => {
  it('completes planning phases and starts executing dispatch-ready runs', async () => {
    const calls: Array<{ url: string; method: string; body?: string }> = [];

    const result = await syncProcessRunState(
      'run-1',
      'dispatch_ready',
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        calls.push({
          url: String(url),
          method: String(init?.method ?? 'GET'),
          body: typeof init?.body === 'string' ? init.body : undefined
        });

        if (String(url).endsWith('/steps') && (init?.method ?? 'GET') === 'GET') {
          return new Response(
            JSON.stringify({
              steps: [
                { step_id: 's1', state_key: 'RECEIVED', status: 'RUNNING' },
                { step_id: 's2', state_key: 'CLASSIFYING', status: 'PENDING' },
                { step_id: 's3', state_key: 'DECOMPOSING', status: 'PENDING' },
                { step_id: 's4', state_key: 'EXECUTING', status: 'PENDING' }
              ]
            }),
            { status: 200 }
          );
        }

        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      }
    );

    assert.equal(result.delegated, true);
    assert.deepEqual(result.transitioned, [
      'RECEIVED:COMPLETED',
      'CLASSIFYING:RUNNING',
      'CLASSIFYING:COMPLETED',
      'DECOMPOSING:RUNNING',
      'DECOMPOSING:COMPLETED',
      'EXECUTING:RUNNING'
    ]);
    assert.ok(calls.some((call) => call.url.endsWith('/runs/run-1/steps') && call.method === 'GET'));
  });

  it('falls back safely when the runtime cannot be reached', async () => {
    const result = await syncProcessRunState(
      'run-2',
      'completed',
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async () => {
        throw new Error('network down');
      }
    );

    assert.equal(result.delegated, false);
    assert.equal(result.warning, 'temporal_worker_sync_unavailable');
  });

  it('keeps runs in decomposition when verification blocks execution', async () => {
    const result = await syncProcessRunState(
      'run-2b',
      'verification_failed',
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        if (String(url).endsWith('/steps') && (init?.method ?? 'GET') === 'GET') {
          return new Response(
            JSON.stringify({
              steps: [
                { step_id: 's1', state_key: 'RECEIVED', status: 'COMPLETED' },
                { step_id: 's2', state_key: 'CLASSIFYING', status: 'COMPLETED' },
                { step_id: 's3', state_key: 'DECOMPOSING', status: 'READY' },
                { step_id: 's4', state_key: 'EXECUTING', status: 'PENDING' }
              ]
            }),
            { status: 200 }
          );
        }
        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      }
    );

    assert.deepEqual(result.transitioned, ['DECOMPOSING:RUNNING']);
  });

  it('registers execute actions as an execution plan', async () => {
    const result = await registerExecutionPlan(
      'run-3',
      {
        run_id: 'run-3',
        thread_id: 'thread-3',
        planner_provider: 'deterministic',
        planner_model: 'deterministic',
        planner_mode: 'deterministic',
        confidence: 0.8,
        requires_clarification: false,
        actions: [
          {
            run_id: 'run-3',
            step_id: 'step_t1',
            task_id: 't1',
            attempt: 1,
            intent: 'email.search',
            skill_id: 'apple-mail-search',
            tool: 'apple-mail-search.search_all',
            operation: 'search_all',
            params: { query: 'find invoice' },
            idempotency_key: 'key-1',
            dependencies: [],
            rationale: 'Search the inbox first.',
            policy: {
              data_class: 'communications',
              sensitivity: 'moderate',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'required',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'redacted_only',
              contains_pii: false,
              retention_class: 'standard',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending'
          }
        ],
        policy_summary: {
          privacy_mode: 'strict',
          data_classes: ['communications'],
          contains_pii: false,
          highest_sensitivity: 'moderate',
          external_model_egress: 'redacted_only',
          requires_consent: true,
          requires_recipient_verification: false,
          human_review_required: false
        },
        risk: {
          impact: 'Low',
          rollback_plan: 'Retry'
        },
        requires_approval: true,
        reasoning: ['deterministic']
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        assert.equal(url, 'http://runtime.local/api/v1/temporal-worker/runs/run-3/execution-plan');
        assert.equal(init?.method, 'POST');
        const payload = JSON.parse(String(init?.body)) as { steps: Array<Record<string, unknown>> };
        assert.equal(payload.steps.length, 1);
        assert.equal(payload.steps[0]?.planner_step_id, 'step_t1');
        return new Response(JSON.stringify({ total: 1 }), { status: 200 });
      }
    );

    assert.deepEqual(result, {
      delegated: true,
      registeredSteps: 1
    });
  });

  it('translates task dependencies into planner step dependencies', async () => {
    await registerExecutionPlan(
      'run-4',
      {
        run_id: 'run-4',
        thread_id: 'thread-4',
        planner_provider: 'deterministic',
        planner_model: 'deterministic',
        planner_mode: 'deterministic',
        confidence: 0.8,
        requires_clarification: false,
        actions: [
          {
            run_id: 'run-4',
            step_id: 'step_t1',
            task_id: 't1',
            attempt: 1,
            intent: 'research.search',
            skill_id: 'tavily',
            tool: 'tavily.search',
            operation: 'search',
            params: { query: 'weather' },
            idempotency_key: 'key-1',
            dependencies: [],
            rationale: 'Search first.',
            policy: {
              data_class: 'general',
              sensitivity: 'low',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'none',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'allow',
              contains_pii: false,
              retention_class: 'ephemeral',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending'
          },
          {
            run_id: 'run-4',
            step_id: 'step_t2',
            task_id: 't2',
            attempt: 1,
            intent: 'research.search',
            skill_id: 'tavily',
            tool: 'tavily.search',
            operation: 'search',
            params: { query: 'forecast' },
            idempotency_key: 'key-2',
            dependencies: ['t1'],
            rationale: 'Then refine.',
            policy: {
              data_class: 'general',
              sensitivity: 'low',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'none',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'allow',
              contains_pii: false,
              retention_class: 'ephemeral',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending'
          }
        ],
        policy_summary: {
          privacy_mode: 'strict',
          data_classes: ['general'],
          contains_pii: false,
          highest_sensitivity: 'low',
          external_model_egress: 'allow',
          requires_consent: false,
          requires_recipient_verification: false,
          human_review_required: false
        },
        risk: {
          impact: 'Low',
          rollback_plan: 'Retry'
        },
        requires_approval: false,
        reasoning: ['deterministic']
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (_url, init) => {
        const payload = JSON.parse(String(init?.body)) as { steps: Array<Record<string, unknown>> };
        assert.deepEqual(payload.steps[1]?.dependencies, ['step_t1']);
        return new Response(JSON.stringify({ total: 2 }), { status: 200 });
      }
    );
  });

  it('maps dependent tasks to every specialist step in a fan-out task', async () => {
    await registerExecutionPlan(
      'run-5',
      {
        run_id: 'run-5',
        thread_id: 'thread-5',
        planner_provider: 'deterministic',
        planner_model: 'deterministic',
        planner_mode: 'deterministic',
        confidence: 0.82,
        requires_clarification: false,
        actions: [
          {
            run_id: 'run-5',
            step_id: 'step_t1__tavily',
            task_id: 't1',
            attempt: 1,
            intent: 'research.search',
            skill_id: 'tavily',
            tool: 'tavily.search',
            operation: 'search',
            params: { query: 'latest A2A standards' },
            idempotency_key: 'key-1-specialist-a',
            dependencies: [],
            step_dependencies: [],
            rationale: 'Search with Tavily.',
            policy: {
              data_class: 'general',
              sensitivity: 'low',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'none',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'allow',
              contains_pii: false,
              retention_class: 'ephemeral',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending',
            fanout_group_id: 'fanout_t1'
          },
          {
            run_id: 'run-5',
            step_id: 'step_t1__brave-search',
            task_id: 't1',
            attempt: 1,
            intent: 'research.search',
            skill_id: 'brave-search',
            tool: 'brave-search.search',
            operation: 'search',
            params: { query: 'latest A2A standards' },
            idempotency_key: 'key-1-specialist-b',
            dependencies: [],
            step_dependencies: [],
            rationale: 'Search with Brave.',
            policy: {
              data_class: 'general',
              sensitivity: 'low',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'none',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'allow',
              contains_pii: false,
              retention_class: 'ephemeral',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending',
            fanout_group_id: 'fanout_t1'
          },
          {
            run_id: 'run-5',
            step_id: 'step_t2',
            task_id: 't2',
            attempt: 1,
            intent: 'research.search',
            skill_id: 'firecrawl-search',
            tool: 'firecrawl-search.search',
            operation: 'search',
            params: { query: 'compare results' },
            idempotency_key: 'key-2',
            dependencies: ['t1'],
            step_dependencies: [],
            rationale: 'Refine after fan-out.',
            policy: {
              data_class: 'general',
              sensitivity: 'low',
              privacy_mode: 'strict',
              legal_basis: 'user_request',
              consent_requirement: 'none',
              recipient_verification: 'not_applicable',
              provenance: 'user_message',
              human_review: 'none',
              external_model_egress: 'allow',
              contains_pii: false,
              retention_class: 'ephemeral',
              allowed_processors: ['brain']
            },
            action_type: 'execute_skill',
            status: 'pending'
          }
        ],
        policy_summary: {
          privacy_mode: 'strict',
          data_classes: ['general'],
          contains_pii: false,
          highest_sensitivity: 'low',
          external_model_egress: 'allow',
          requires_consent: false,
          requires_recipient_verification: false,
          human_review_required: false
        },
        risk: {
          impact: 'Low',
          rollback_plan: 'Retry'
        },
        requires_approval: false,
        reasoning: ['deterministic']
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (_url, init) => {
        const payload = JSON.parse(String(init?.body)) as { steps: Array<Record<string, unknown>> };
        assert.deepEqual(payload.steps[2]?.dependencies, ['step_t1__tavily', 'step_t1__brave-search']);
        return new Response(JSON.stringify({ total: 3 }), { status: 200 });
      }
    );
  });

  it('annotates runs with verification failures', async () => {
    const result = await annotateRunVerification(
      'run-6',
      {
        valid: false,
        issues: ['missing_policy_for_step_t1'],
        warnings: ['process_response_is_dispatch_only_until_real_skill_results_arrive']
      },
      {
        temporalWorkerBaseUrl: 'http://runtime.local',
        temporalWorkerTimeoutMs: 1000
      },
      async (url, init) => {
        assert.equal(url, 'http://runtime.local/api/v1/temporal-worker/runs/run-6/metadata');
        assert.equal(init?.method, 'POST');
        const payload = JSON.parse(String(init?.body)) as { metadata: Record<string, unknown> };
        assert.equal(payload.metadata.verification_valid, false);
        assert.deepEqual(payload.metadata.verification_issues, ['missing_policy_for_step_t1']);
        return new Response(JSON.stringify({ ok: true }), { status: 200 });
      }
    );

    assert.deepEqual(result, { delegated: true });
  });
});
