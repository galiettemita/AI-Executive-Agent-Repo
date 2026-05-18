import { z } from 'zod';

const ActionSchema = z.enum(['generate_shortcut', 'list_shortcuts', 'install_shortcut']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    shortcut_name: z.string().min(2).max(180).optional(),
    description: z.string().min(2).max(500).optional(),
    steps: z.array(z.string().min(1).max(240)).min(1).max(30).optional(),
    shortcut_id: z.string().min(3).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_shortcut' && (!value.shortcut_name || !value.steps || value.steps.length === 0)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SHORTCUTS_GENERATOR_STEPS_REQUIRED' });
    }
    if (value.action === 'install_shortcut' && !value.shortcut_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SHORTCUTS_GENERATOR_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('shortcuts-generator'),
    action: ActionSchema,
    shortcuts: z.array(
      z
        .object({
          shortcut_id: z.string().min(3).max(120),
          name: z.string().min(1).max(180),
          status: z.enum(['generated', 'installed']),
          install_url: z.string().url().optional(),
          step_count: z.number().int().min(0).max(100)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
