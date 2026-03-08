import { z } from 'zod';

const ActionSchema = z.enum([
  'add_items',
  'remove_items',
  'list_items',
  'organize_by_section',
  'clear_list'
]);

const SectionSchema = z.enum(['produce', 'dairy', 'protein', 'pantry', 'frozen', 'household', 'other']);

const ItemInputSchema = z.object({
  name: z.string().min(1).max(120),
  quantity: z.number().int().min(1).max(50).optional(),
  section: SectionSchema.optional()
});

const ItemOutputSchema = z.object({
  item_id: z.string().min(2).max(80),
  name: z.string().min(1).max(120),
  quantity: z.number().int().min(1).max(50),
  section: SectionSchema,
  checked: z.boolean()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    list_id: z.string().min(2).max(80).optional(),
    items: z.array(ItemInputSchema).max(200).optional(),
    item_ids: z.array(z.string().min(2).max(80)).max(200).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'add_items' && !value.items?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GROCERY_LIST_ITEMS_REQUIRED' });
    }

    if (value.action === 'remove_items' && !value.item_ids?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'GROCERY_LIST_ITEM_IDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('grocery-list'),
    action: ActionSchema,
    list_id: z.string().min(2).max(80),
    items: z.array(ItemOutputSchema).max(400),
    total_items: z.number().int().min(0)
  })
  .strict();
