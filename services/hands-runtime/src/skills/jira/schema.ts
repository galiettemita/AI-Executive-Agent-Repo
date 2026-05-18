import { z } from 'zod';

const IssueSchema = z.object({
  issue_key: z.string(),
  summary: z.string(),
  status: z.string(),
  project_key: z.string()
});

export const InputSchema = z
  .object({
    action: z.enum(['issue_list', 'issue_create', 'issue_transition']),
    project_key: z.string().min(2).max(20).optional(),
    issue_key: z.string().min(2).max(40).optional(),
    summary: z.string().min(1).max(300).optional(),
    description: z.string().max(5000).optional(),
    transition_to: z.string().max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('jira'),
    action: z.enum(['issue_list', 'issue_create', 'issue_transition']),
    issue_key: z.string().optional(),
    issues: z.array(IssueSchema).optional()
  })
  .strict();
