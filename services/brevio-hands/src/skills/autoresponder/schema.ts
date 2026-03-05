import { z } from 'zod';

const ActionSchema = z.enum(['enable', 'disable', 'intercept']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    ruleset_name: z.string().min(2).max(120).optional(),
    incoming_text: z.string().min(1).max(4096).optional(),
    channel: z.enum(['whatsapp', 'imessage']).optional(),
    delegation_enabled: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'intercept' && !value.incoming_text?.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'AUTORESPONDER_INTERCEPT_TEXT_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('autoresponder'),
    action: ActionSchema,
    status: z.enum(['enabled', 'disabled', 'responded']),
    delegated_to_brain: z.boolean(),
    response_text: z.string().min(1).max(4096).optional(),
    latency_budget_ms: z.literal(8000)
  })
  .strict();
