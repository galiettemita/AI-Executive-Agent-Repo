export interface ExecutionRefs {
  request_id?: string;
  run_id?: string;
  task_id?: string;
  step_id?: string;
  attempt?: number;
}

function asString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export function parseExecutionRefs(payload: Record<string, unknown>): ExecutionRefs {
  const attempt = payload.attempt;

  return {
    request_id: asString(payload.request_id),
    run_id: asString(payload.run_id),
    task_id: asString(payload.task_id),
    step_id: asString(payload.step_id),
    attempt: typeof attempt === 'number' && Number.isInteger(attempt) && attempt > 0 ? attempt : undefined
  };
}

export function applyExecutionRefs<T extends Record<string, unknown>>(payload: T, refs: ExecutionRefs): T & ExecutionRefs {
  return {
    ...payload,
    ...(refs.request_id ? { request_id: refs.request_id } : {}),
    ...(refs.run_id ? { run_id: refs.run_id } : {}),
    ...(refs.task_id ? { task_id: refs.task_id } : {}),
    ...(refs.step_id ? { step_id: refs.step_id } : {}),
    ...(refs.attempt ? { attempt: refs.attempt } : {})
  };
}
