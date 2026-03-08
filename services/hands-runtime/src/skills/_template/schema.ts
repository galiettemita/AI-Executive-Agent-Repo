import { z } from 'zod';

export const TemplateSkillInputSchema = z.object({
  prompt: z.string().min(1)
});

export const TemplateSkillOutputSchema = z.object({
  ok: z.boolean()
});
