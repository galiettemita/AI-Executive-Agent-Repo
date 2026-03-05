import { z } from 'zod';

const ActionSchema = z.enum(['analyze_file', 'export_asset', 'audit_accessibility']);
const FormatSchema = z.enum(['png', 'svg', 'pdf']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    file_key: z.string().min(3).max(120).optional(),
    node_id: z.string().min(2).max(120).optional(),
    format: FormatSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.file_key) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'FIGMA_FILE_KEY_REQUIRED' });
    }
    if (value.action === 'export_asset' && !value.node_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'FIGMA_NODE_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('figma'),
    action: ActionSchema,
    file_key: z.string().min(3).max(120),
    findings: z.array(z.string().min(3).max(300)).max(20),
    export_url: z.string().url().optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
