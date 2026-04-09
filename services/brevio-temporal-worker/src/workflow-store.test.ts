import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { describe, it } from 'node:test';

import { WorkflowStore } from './workflow-store.js';

function testStorePath(name: string): string {
  const dir = mkdtempSync(path.join(tmpdir(), 'brevio-temporal-worker-'));
  return path.join(dir, `${name}.json`);
}

describe('WorkflowStore', () => {
  it('persists created runs with task and step snapshots', () => {
    const storePath = testStorePath('persist');
    const now = '2026-04-09T16:30:00.000Z';
    const store = new WorkflowStore(storePath);

    const run = store.createRun({
      run_id: 'run-persist-1',
      workflow_id: 'msg-1',
      workflow_type: 'message-processing',
      user_id: 'user-1',
      status: 'COMPLETED',
      current_state: 'COMPLETED',
      started_at: now,
      completed_at: now,
      metadata: { message_id: 'msg-1' },
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'COMPLETED', title: 'COMPLETED', status: 'COMPLETED' }
      ]
    });

    assert.equal(run.run_id, 'run-persist-1');
    assert.equal(store.listTasks(run.run_id).length, 2);
    assert.equal(store.listSteps(run.run_id).length, 2);

    const reloaded = new WorkflowStore(storePath);
    assert.equal(reloaded.getRun('run-persist-1')?.workflow_id, 'msg-1');
    assert.equal(reloaded.listTasks('run-persist-1').length, 2);
    assert.equal(reloaded.listSteps('run-persist-1').length, 2);
  });

  it('promotes the next step and resumes paused runs', () => {
    const store = new WorkflowStore(testStorePath('resume'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-resume-1',
      workflow_id: 'msg-2',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'DECOMPOSING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'CLASSIFYING', title: 'CLASSIFYING', status: 'COMPLETED' },
        { state_key: 'DECOMPOSING', title: 'DECOMPOSING', status: 'RUNNING' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'PENDING' }
      ]
    });

    const currentStep = store.listSteps(run.run_id).find((step) => step.state_key === 'DECOMPOSING');
    assert.ok(currentStep);

    store.transitionStep({
      run_id: run.run_id,
      step_id: currentStep.step_id,
      status: 'COMPLETED'
    });

    const afterCompletion = store.listSteps(run.run_id);
    const executing = afterCompletion.find((step) => step.state_key === 'EXECUTING');
    assert.equal(executing?.status, 'READY');

    const resumedRun = store.resumeRun(run.run_id);
    assert.equal(resumedRun.status, 'RUNNING');
    assert.equal(resumedRun.current_state, 'EXECUTING');
    assert.equal(store.listSteps(run.run_id).find((step) => step.state_key === 'EXECUTING')?.status, 'RUNNING');
    assert.equal(store.listSteps(run.run_id).find((step) => step.state_key === 'EXECUTING')?.attempt, 1);
  });

  it('marks runs terminal when a step fails', () => {
    const store = new WorkflowStore(testStorePath('fail'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-fail-1',
      workflow_id: 'msg-3',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' }
      ]
    });

    const step = store.listSteps(run.run_id).find((candidate) => candidate.state_key === 'EXECUTING');
    assert.ok(step);

    store.transitionStep({
      run_id: run.run_id,
      step_id: step.step_id,
      status: 'FAILED',
      error: {
        code: 'EXTERNAL_TIMEOUT',
        message: 'execution timed out'
      }
    });

    const updatedRun = store.getRun(run.run_id);
    assert.equal(updatedRun?.status, 'FAILED');
    assert.equal(updatedRun?.current_state, 'EXECUTING');
    assert.equal(store.listSteps(run.run_id).find((candidate) => candidate.state_key === 'EXECUTING')?.last_error?.code, 'EXTERNAL_TIMEOUT');
  });

  it('registers execution plans, promotes dependency-ready steps, and gates aggregation on planned actions', () => {
    const store = new WorkflowStore(testStorePath('execution-plan'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-plan-1',
      workflow_id: 'msg-4',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'CLASSIFYING', title: 'CLASSIFYING', status: 'COMPLETED' },
        { state_key: 'DECOMPOSING', title: 'DECOMPOSING', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' },
        { state_key: 'AGGREGATING', title: 'AGGREGATING', status: 'PENDING' }
      ]
    });

    const registered = store.registerExecutionPlan(run.run_id, [
      {
        planner_step_id: 'step_t1',
        planner_task_id: 't1',
        title: 'Search inbox',
        skill_id: 'apple-mail-search',
        operation: 'search_all'
      },
      {
        planner_step_id: 'step_t2',
        planner_task_id: 't2',
        title: 'Draft reply',
        skill_id: 'apple-mail',
        operation: 'reply',
        dependencies: ['step_t1']
      }
    ]);

    assert.equal(registered.length, 2);
    assert.equal(store.getPlannerStep(run.run_id, 'step_t1')?.status, 'READY');
    assert.equal(store.getPlannerStep(run.run_id, 'step_t2')?.status, 'PENDING');

    store.transitionPlannerStep({
      run_id: run.run_id,
      planner_step_id: 'step_t1',
      status: 'RUNNING'
    });
    store.transitionPlannerStep({
      run_id: run.run_id,
      planner_step_id: 'step_t1',
      status: 'COMPLETED'
    });

    assert.equal(store.getPlannerStep(run.run_id, 'step_t2')?.status, 'READY');

    store.transitionPlannerStep({
      run_id: run.run_id,
      planner_step_id: 'step_t2',
      status: 'RUNNING'
    });
    store.transitionPlannerStep({
      run_id: run.run_id,
      planner_step_id: 'step_t2',
      status: 'COMPLETED'
    });

    const aggregating = store.listSteps(run.run_id).find((step) => step.state_key === 'AGGREGATING');
    assert.equal(aggregating?.status, 'READY');
  });

  it('registers the same execution plan idempotently', () => {
    const store = new WorkflowStore(testStorePath('execution-plan-idempotent'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-plan-2',
      workflow_id: 'msg-5',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' },
        { state_key: 'AGGREGATING', title: 'AGGREGATING', status: 'PENDING' }
      ]
    });

    store.registerExecutionPlan(run.run_id, [
      {
        planner_step_id: 'step_t1',
        planner_task_id: 't1',
        title: 'Do thing',
        skill_id: 'todoist',
        operation: 'create'
      }
    ]);
    store.registerExecutionPlan(run.run_id, [
      {
        planner_step_id: 'step_t1',
        planner_task_id: 't1',
        title: 'Do thing',
        skill_id: 'todoist',
        operation: 'create'
      }
    ]);

    const planSteps = store.listSteps(run.run_id).filter((step) => step.metadata?.phase === 'EXECUTING' && step.metadata?.planner_step_id === 'step_t1');
    assert.equal(planSteps.length, 1);
  });

  it('groups multiple specialist planner steps under one task record', () => {
    const store = new WorkflowStore(testStorePath('execution-plan-fanout'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-plan-fanout-1',
      workflow_id: 'msg-7',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' },
        { state_key: 'AGGREGATING', title: 'AGGREGATING', status: 'PENDING' }
      ]
    });

    store.registerExecutionPlan(run.run_id, [
      {
        planner_step_id: 'step_t1__tavily',
        planner_task_id: 't1',
        title: 'Research with Tavily',
        skill_id: 'tavily',
        operation: 'search'
      },
      {
        planner_step_id: 'step_t1__brave',
        planner_task_id: 't1',
        title: 'Research with Brave',
        skill_id: 'brave-search',
        operation: 'search'
      }
    ]);

    const plannerTask = store.listTasks(run.run_id).find((task) => task.metadata?.planner_task_id === 't1');
    assert.ok(plannerTask);
    assert.equal(plannerTask?.step_ids.length, 2);
    assert.equal(store.listTasks(run.run_id).filter((task) => task.metadata?.planner_task_id === 't1').length, 1);
  });

  it('cancels a task by dead-lettering its step and run', () => {
    const store = new WorkflowStore(testStorePath('task-cancel'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-cancel-1',
      workflow_id: 'msg-6',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' }
      ]
    });

    const executingTask = store.listTasks(run.run_id).find((task) => task.name === 'EXECUTING');
    assert.ok(executingTask);

    const cancelled = store.cancelTask(executingTask.task_id);
    assert.equal(cancelled.status, 'DEAD_LETTER');
    assert.equal(store.getRun(run.run_id)?.status, 'DEAD_LETTER');
  });

  it('rejects invalid transitions before dependencies are ready', () => {
    const store = new WorkflowStore(testStorePath('invalid-transition'));
    const now = '2026-04-09T16:30:00.000Z';

    const run = store.createRun({
      run_id: 'run-invalid-1',
      workflow_id: 'msg-8',
      workflow_type: 'message-processing',
      status: 'RUNNING',
      current_state: 'EXECUTING',
      started_at: now,
      metadata: {},
      steps: [
        { state_key: 'RECEIVED', title: 'RECEIVED', status: 'COMPLETED' },
        { state_key: 'EXECUTING', title: 'EXECUTING', status: 'RUNNING' },
        { state_key: 'AGGREGATING', title: 'AGGREGATING', status: 'PENDING' }
      ]
    });

    store.registerExecutionPlan(run.run_id, [
      {
        planner_step_id: 'step_t1',
        planner_task_id: 't1',
        title: 'Search first',
        skill_id: 'tavily',
        operation: 'search'
      },
      {
        planner_step_id: 'step_t2',
        planner_task_id: 't2',
        title: 'Draft second',
        skill_id: 'apple-mail',
        operation: 'reply',
        dependencies: ['step_t1']
      }
    ]);

    assert.throws(
      () =>
        store.transitionPlannerStep({
          run_id: run.run_id,
          planner_step_id: 'step_t2',
          status: 'COMPLETED'
        }),
      /step_dependencies_unresolved|invalid_step_transition/
    );
  });
});
