import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { decomposeTask } from './decompose.js';

describe('decomposeTask', () => {
  it('builds sequential dependencies when the request contains ordering cues', () => {
    const output = decomposeTask('book a flight and then email me the itinerary', [], true);

    assert.equal(output.execution_order, 'sequential');
    assert.equal(output.tasks.length, 2);
    assert.deepEqual(output.tasks[1]?.dependencies, ['t1']);
  });

  it('builds parallel tasks for independent actions', () => {
    const output = decomposeTask('add task to call mom and save this note', [], true);

    assert.equal(output.execution_order, 'parallel');
    assert.equal(output.tasks.length, 2);
    assert.deepEqual(output.tasks[0]?.dependencies, []);
    assert.deepEqual(output.tasks[1]?.dependencies, []);
  });

  it('preserves mixed dependencies across parallel batches', () => {
    const output = decomposeTask('search inbox and find the attachment, then draft a reply', [], true);

    assert.equal(output.execution_order, 'mixed');
    assert.equal(output.tasks.length, 3);
    assert.deepEqual(output.tasks[0]?.dependencies, []);
    assert.deepEqual(output.tasks[1]?.dependencies, []);
    assert.deepEqual(output.tasks[2]?.dependencies, ['t1', 't2']);
  });

  it('rejects empty requests', () => {
    assert.throws(() => decomposeTask('', [], true), /TASK_GRAPH_INVALID: request_required/);
  });
});
