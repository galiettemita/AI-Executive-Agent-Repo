export type TrelloAction = 'card_list' | 'card_create' | 'card_move';

export interface TrelloInput {
  action: TrelloAction;
  board_id?: string;
  list_id?: string;
  card_id?: string;
  name?: string;
  desc?: string;
  target_list_id?: string;
}

export interface TrelloCard {
  card_id: string;
  name: string;
  list_id: string;
  board_id: string;
}

export interface TrelloOutput {
  provider: 'trello';
  action: TrelloAction;
  card_id?: string;
  cards?: TrelloCard[];
}
