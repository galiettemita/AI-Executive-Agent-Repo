import { z } from 'zod';

const IssueSchema = z.object({
  issue_id: z.string(),
  title: z.string(),
  status: z.string(),
  team_id: z.string()
});

export const InputSchema = z
  .object({
    action: z.enum(['issue_list', 'issue_create', 'issue_update']),
    team_id: z.string().min(2).max(80).optional(),
    issue_id: z.string().min(2).max(80).optional(),
    title: z.string().min(1).max(300).optional(),
    description: z.string().max(5000).optional(),
    status: z.string().max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('linear'),
    action: z.enum(['issue_list', 'issue_create', 'issue_update']),
    issue_id: z.string().optional(),
    issues: z.array(IssueSchema).optional()
  })
  .strict();
