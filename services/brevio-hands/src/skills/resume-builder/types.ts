export interface ResumeBuilderInput {
  action: 'generate' | 'tailor' | 'score';
  role?: string;
  experience_bullets?: string[];
  job_description?: string;
  resume_markdown?: string;
}

export interface ResumeBuilderOutput {
  provider: 'resume-builder';
  action: ResumeBuilderInput['action'];
  resume_markdown?: string;
  score?: number;
  recommendations?: string[];
}
