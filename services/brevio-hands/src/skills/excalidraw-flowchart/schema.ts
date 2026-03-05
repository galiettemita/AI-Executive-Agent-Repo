import { z } from 'zod';

const ActionSchema = z.enum(['generate_flowchart', 'update_flowchart', 'export_svg']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    description: z.string().min(2).max(1000).optional(),
    flowchart_id: z.string().min(3).max(120).optional(),
    nodes: z.array(z.string().min(1).max(120)).min(1).max(50).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_flowchart' && !value.description) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'EXCALIDRAW_DESCRIPTION_REQUIRED' });
    }
    if (value.action === 'update_flowchart' && (!value.flowchart_id || !value.nodes)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'EXCALIDRAW_UPDATE_FIELDS_REQUIRED' });
    }
    if (value.action === 'export_svg' && !value.flowchart_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'EXCALIDRAW_FLOWCHART_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('excalidraw-flowchart'),
    action: ActionSchema,
    flowchart_id: z.string().min(3).max(120),
    nodes: z.array(
      z
        .object({
          id: z.string().min(1).max(120),
          label: z.string().min(1).max(240)
        })
        .strict()
    ),
    edges: z.array(
      z
        .object({
          from: z.string().min(1).max(120),
          to: z.string().min(1).max(120)
        })
        .strict()
    ),
    export_url: z.string().url().optional(),
    summary: z.string().min(10).max(4096)
  })
  .strict();
