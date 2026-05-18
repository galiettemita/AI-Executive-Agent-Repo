import { z } from 'zod';

const ActionSchema = z.enum(['review_contract', 'summarize_risks']);
const ContractTypeSchema = z.enum(['msa', 'nda', 'lease', 'employment', 'other']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    contract_text: z.string().min(50).max(40000).optional(),
    contract_type: ContractTypeSchema.optional(),
    jurisdiction: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.contract_text) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CONTRACT_REVIEWER_TEXT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('contract-reviewer'),
    action: ActionSchema,
    overall_risk: z.enum(['low', 'medium', 'high']),
    risk_items: z.array(
      z
        .object({
          clause: z.string().min(2).max(240),
          severity: z.enum(['low', 'medium', 'high']),
          rationale: z.string().min(5).max(500)
        })
        .strict()
    ),
    must_review_clauses: z.array(z.string().min(2).max(240)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();
