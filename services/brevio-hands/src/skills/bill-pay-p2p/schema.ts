import { z } from 'zod';

const ActionSchema = z.enum(['list_payees', 'create_payment', 'payment_status', 'cancel_payment']);

const PayeeSchema = z.object({
  payee_id: z.string(),
  name: z.string(),
  last4: z.string().optional()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    payee_id: z.string().min(2).max(120).optional(),
    amount_cents: z.number().int().positive().max(500000000).optional(),
    memo: z.string().min(1).max(280).optional(),
    payment_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_payment' && (!value.payee_id || !value.amount_cents)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'BILL_PAY_PAYMENT_FIELDS_REQUIRED'
      });
    }

    if ((value.action === 'payment_status' || value.action === 'cancel_payment') && !value.payment_id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'BILL_PAY_PAYMENT_ID_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('bill-pay-p2p'),
    action: ActionSchema,
    payees: z.array(PayeeSchema).optional(),
    payment_id: z.string().optional(),
    status: z.enum(['pending', 'scheduled', 'completed', 'cancelled']).optional(),
    partnership_status: z.literal('awaiting_api_partnership')
  })
  .strict();
