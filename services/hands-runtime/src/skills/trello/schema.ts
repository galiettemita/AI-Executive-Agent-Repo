import { z } from 'zod';

const CardSchema = z.object({
  card_id: z.string(),
  name: z.string(),
  list_id: z.string(),
  board_id: z.string()
});

export const InputSchema = z
  .object({
    action: z.enum(['card_list', 'card_create', 'card_move']),
    board_id: z.string().min(2).max(120).optional(),
    list_id: z.string().min(2).max(120).optional(),
    card_id: z.string().min(2).max(120).optional(),
    name: z.string().min(1).max(300).optional(),
    desc: z.string().max(5000).optional(),
    target_list_id: z.string().min(2).max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('trello'),
    action: z.enum(['card_list', 'card_create', 'card_move']),
    card_id: z.string().optional(),
    cards: z.array(CardSchema).optional()
  })
  .strict();
