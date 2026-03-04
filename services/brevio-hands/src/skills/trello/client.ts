import { createHash } from 'node:crypto';

import type { TrelloCard, TrelloInput, TrelloOutput } from './types.js';

const CARDS: TrelloCard[] = [
  {
    card_id: 'trl_001',
    name: 'Finalize launch checklist',
    list_id: 'todo',
    board_id: 'exec_board'
  },
  {
    card_id: 'trl_002',
    name: 'Draft stakeholder update',
    list_id: 'doing',
    board_id: 'exec_board'
  }
];

function cardId(name: string): string {
  return `trl_${createHash('sha256').update(name).digest('hex').slice(0, 8)}`;
}

export async function runClient(input: TrelloInput): Promise<TrelloOutput> {
  if (input.action === 'card_list') {
    return {
      provider: 'trello',
      action: 'card_list',
      cards: CARDS.filter(
        (card) => (!input.board_id || card.board_id === input.board_id) && (!input.list_id || card.list_id === input.list_id)
      )
    };
  }

  if (input.action === 'card_create') {
    if (!input.board_id || !input.list_id || !input.name) {
      throw new Error('TRELLO_CREATE_FIELDS_REQUIRED');
    }

    return {
      provider: 'trello',
      action: 'card_create',
      card_id: cardId(`${input.board_id}|${input.list_id}|${input.name}`),
      cards: [
        {
          card_id: cardId(`${input.board_id}|${input.list_id}|${input.name}`),
          name: input.name,
          list_id: input.list_id,
          board_id: input.board_id
        }
      ]
    };
  }

  if (!input.card_id || !input.target_list_id) {
    throw new Error('TRELLO_MOVE_FIELDS_REQUIRED');
  }

  return {
    provider: 'trello',
    action: 'card_move',
    card_id: input.card_id,
    cards: [
      {
        card_id: input.card_id,
        name: input.name ?? 'Moved card',
        list_id: input.target_list_id,
        board_id: input.board_id ?? 'exec_board'
      }
    ]
  };
}
