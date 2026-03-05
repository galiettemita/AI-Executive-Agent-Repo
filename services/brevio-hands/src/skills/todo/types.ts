export type TodoAction = 'list' | 'add' | 'complete' | 'delete';

export interface TodoInput {
  action: TodoAction;
  item_id?: string;
  content?: string;
  due?: string;
}

export interface TodoItem {
  item_id: string;
  content: string;
  due?: string;
  completed: boolean;
}

export interface TodoOutput {
  provider: 'todo';
  action: TodoAction;
  item_id?: string;
  items?: TodoItem[];
}
