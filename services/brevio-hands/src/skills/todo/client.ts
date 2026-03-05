import { createHash } from 'node:crypto';

import type { TodoInput, TodoItem, TodoOutput } from './types.js';

const ITEMS: TodoItem[] = [
  {
    item_id: 'todo_001',
    content: 'Review hiring plan',
    due: 'tomorrow 9am',
    completed: false
  },
  {
    item_id: 'todo_002',
    content: 'Prepare investor update',
    due: 'friday 5pm',
    completed: false
  }
];

function itemId(content: string): string {
  return `todo_${createHash('sha256').update(content).digest('hex').slice(0, 10)}`;
}

export async function runClient(input: TodoInput): Promise<TodoOutput> {
  if (input.action === 'list') {
    return {
      provider: 'todo',
      action: 'list',
      items: ITEMS
    };
  }

  if (input.action === 'add') {
    if (!input.content) {
      throw new Error('TODO_CONTENT_REQUIRED');
    }
    return {
      provider: 'todo',
      action: 'add',
      item_id: itemId(input.content),
      items: [
        {
          item_id: itemId(input.content),
          content: input.content,
          due: input.due,
          completed: false
        }
      ]
    };
  }

  if (!input.item_id) {
    throw new Error('TODO_ITEM_ID_REQUIRED');
  }

  if (input.action === 'complete') {
    return {
      provider: 'todo',
      action: 'complete',
      item_id: input.item_id,
      items: [
        {
          item_id: input.item_id,
          content: 'Completed item',
          completed: true
        }
      ]
    };
  }

  return {
    provider: 'todo',
    action: 'delete',
    item_id: input.item_id
  };
}
