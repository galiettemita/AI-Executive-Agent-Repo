export type ReactEmailSkillsAction = 'render_template' | 'preview_message';

export interface ReactEmailSkillsInput {
  action: ReactEmailSkillsAction;
  template_id?: string;
  subject?: string;
  variables?: Record<string, string>;
  preview_to?: string;
}

export interface ReactEmailSkillsOutput {
  provider: 'react-email-skills';
  action: ReactEmailSkillsAction;
  html: string;
  text: string;
  preview_id: string;
  summary: string;
}
