import { z } from 'zod';

const ActionSchema = z.enum(['get_score', 'get_schedule']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    league: z.enum(['nba', 'nfl', 'mlb', 'nhl', 'epl']).optional(),
    team: z.string().min(2).max(120).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.league) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPORTS_TICKER_LEAGUE_REQUIRED' });
    }
    if (value.action === 'get_score' && !value.team) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'SPORTS_TICKER_TEAM_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('sports-ticker'),
    action: ActionSchema,
    items: z.array(
      z
        .object({
          title: z.string().min(2).max(200),
          status: z.string().min(2).max(200)
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();
