import { z } from 'zod';

const ActionSchema = z.enum(['list_actions', 'trigger_action']);

const ActionDescriptorSchema = z.object({
  action_key: z.string().min(2).max(120),
  app_name: z.string().min(2).max(120),
  display_name: z.string().min(2).max(200),
  callback_url_template: z.string().url()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    action_key: z.string().min(2).max(120).optional(),
    app_name: z.string().min(2).max(120).optional(),
    parameters: z.record(z.string()).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'trigger_action' && !value.action_key) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'ALTER_ACTIONS_ACTION_KEY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('alter-actions'),
    action: ActionSchema,
    actions: z.array(ActionDescriptorSchema).max(200),
    triggered_action: z.string().min(2).max(120).optional(),
    callback_url: z.string().url().optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
