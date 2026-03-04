import { z } from 'zod';

const ActionSchema = z.enum(['parse_recipe', 'sync_todoist']);
const SectionSchema = z.enum(['produce', 'dairy', 'protein', 'pantry', 'frozen', 'household', 'other']);

const RecipeItemInputSchema = z.object({
  item: z.string().min(1).max(120),
  quantity: z.string().min(1).max(40).optional()
});

const RecipeItemOutputSchema = z.object({
  item: z.string().min(1).max(120),
  quantity: z.string().min(1).max(40).optional(),
  section: SectionSchema
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    recipe_title: z.string().min(2).max(200).optional(),
    recipe_text: z.string().min(10).max(12000).optional(),
    recipe_items: z.array(RecipeItemInputSchema).max(200).optional(),
    project_id: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'parse_recipe' && !value.recipe_text?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RECIPE_TO_LIST_TEXT_REQUIRED' });
    }

    if (value.action === 'sync_todoist' && !value.recipe_items?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'RECIPE_TO_LIST_ITEMS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('recipe-to-list'),
    action: ActionSchema,
    recipe_title: z.string().min(2).max(200),
    normalized_items: z.array(RecipeItemOutputSchema).max(200),
    task_ids: z.array(z.string().min(2).max(120)).optional(),
    project_name: z.string().min(2).max(120).optional()
  })
  .strict();
