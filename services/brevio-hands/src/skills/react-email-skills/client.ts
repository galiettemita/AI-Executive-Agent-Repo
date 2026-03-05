import type { ReactEmailSkillsInput, ReactEmailSkillsOutput } from './types.js';

export async function runClient(input: ReactEmailSkillsInput): Promise<ReactEmailSkillsOutput> {
  const subject = input.subject ?? 'Brevio Update';
  return {
    provider: 'react-email-skills',
    action: input.action,
    html: `<html><body><h1>${subject}</h1><p>Preview template ${input.template_id}</p></body></html>`,
    text: `${subject}\n\nPreview template ${input.template_id}`,
    preview_id: `preview-${input.template_id}`,
    summary: `Rendered email template ${input.template_id}.`
  };
}
