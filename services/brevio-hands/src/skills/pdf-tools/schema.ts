import { z } from 'zod';

const ActionSchema = z.enum(['extract_text', 'merge', 'split']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    files: z.array(z.string().min(1)).min(1).max(20),
    page_range: z.string().regex(/^\d+-\d+$/u).optional(),
    output_name: z.string().min(1).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'merge' && value.files.length < 2) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PDF_TOOLS_MERGE_FILES_REQUIRED' });
    }

    if (value.action === 'split' && !value.page_range) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PDF_TOOLS_PAGE_RANGE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('pdf-tools'),
    action: ActionSchema,
    output_path: z.string(),
    pages_processed: z.number().int().positive(),
    extracted_text_preview: z.string().optional()
  })
  .strict();
