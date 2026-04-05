export type Channel = 'WHATSAPP' | 'IMESSAGE' | 'API';

export type UserTier = 'free' | 'pro' | 'enterprise' | 'admin' | 'service';

export type DeploymentMode = 'cloud' | 'local_mac' | 'mcp' | 'terminal';

export type PlannerProvider = 'deterministic' | 'openai_responses';

export type ExecutionStatus = 'dispatch_ready' | 'completed' | 'clarification_required';

export type TaskStatus = 'planned' | 'clarify';

export interface BrainConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  disambiguationConfigPath: string;
  plannerProvider: PlannerProvider;
  plannerModel: string;
  plannerFallbackModel: string;
  plannerTimeoutMs: number;
  plannerBaseUrl: string;
}

export interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  userId?: string;
}

export interface UserPreferences {
  email_provider?: 'google' | 'microsoft' | 'apple' | 'imap' | 'none';
  music_provider?: 'spotify' | 'apple_music' | 'youtube_music' | 'none';
  task_app?:
    | 'todoist'
    | 'things'
    | 'ticktick'
    | 'omnifocus'
    | 'trello'
    | 'asana'
    | 'linear'
    | 'jira'
    | 'clickup'
    | 'apple_reminders'
    | 'none';
  notes_app?: 'apple_notes' | 'notion' | 'bear' | 'obsidian' | 'craft' | 'google_keep' | 'reflect' | 'none';
  finance_app?: 'ynab' | 'monarch' | 'copilot' | 'none';
  has_edge_agent?: boolean;
}

export interface UserProfile {
  timezone?: string;
  locale?: string;
  enabled_skills?: string[];
  recent_intents?: string[];
  preferences?: UserPreferences;
  communication_style?: 'concise' | 'balanced' | 'detailed';
}

export interface ReasoningContext {
  time_of_day?: string;
  day_of_week?: string;
  active_session_minutes?: number;
}

export interface IntentClassificationInput {
  message_text: string;
  user_profile?: UserProfile;
  user_preferences?: UserPreferences;
  deployment_mode?: DeploymentMode;
  user_tier?: UserTier;
  channel?: Channel;
  context?: ReasoningContext;
}

export interface IntentEvidence {
  intent: string;
  matched_keywords: string[];
  score: number;
}

export interface IntentClassificationOutput {
  intent: string;
  confidence: number;
  skills: string[];
  requires_decomposition: boolean;
  reasoning: string;
  clarification_required: boolean;
  blocked_skills: string[];
  evidence: IntentEvidence[];
  suggested_clarification?: string;
  operation?: string;
}

export interface DisambiguationRequest {
  message_text: string;
  intent?: string;
  candidate_skills?: string[];
  deployment_mode?: DeploymentMode;
  user_tier?: UserTier;
  user_preferences?: UserPreferences;
  enabled_skills?: string[];
  allow_multi_intent?: boolean;
}

export interface DisambiguationResponse {
  resolved_skills: string[];
  group_hits: string[];
  blocked_skills: string[];
  clarification_required: boolean;
  reasoning: string[];
}

export interface TaskDescriptor {
  id: string;
  goal: string;
  intent: string;
  skill_id?: string;
  input: Record<string, unknown>;
  dependencies: string[];
  priority: number;
  status: TaskStatus;
  reasoning: string;
}

export interface TaskDecompositionOutput {
  tasks: TaskDescriptor[];
  execution_order: 'parallel' | 'sequential' | 'mixed';
  requires_clarification: boolean;
  reasoning: string[];
}

export interface SkillResult {
  skill_id: string;
  status: 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT';
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
  source?: 'hands' | 'external';
}

export interface AggregationRequest {
  skill_results: SkillResult[];
  user_profile?: {
    communication_style?: 'concise' | 'balanced' | 'detailed';
  };
  channel?: Channel;
}

export interface AggregationResponse {
  response_text: string;
  suggested_actions: string[];
  follow_up_scheduled: boolean;
  completion_ratio: number;
  warnings: string[];
}

export interface DisambiguationRuleConfig {
  group: string;
  canonical?: string;
  aliases?: string[];
  fallback?: string;
  cloud?: string;
  local_mac?: string;
  terminal?: string;
  analytics?: string;
  track?: string;
  find?: string;
  free_tier?: string;
  crud?: string;
  search?: string;
  by_preference?: Record<string, string>;
  delegates?: string[];
  international?: string;
  carriers_17track?: string;
  austrian_post?: string;
  navigate?: string;
  near_me?: string;
  find_all?: string;
  simple_nearby?: string;
  summarize?: string;
  download?: string;
}

export type DisambiguationRules = Record<string, DisambiguationRuleConfig>;

export interface PlannedAction {
  step_id: string;
  task_id: string;
  intent: string;
  skill_id?: string;
  tool?: string;
  operation: string;
  params: Record<string, unknown>;
  idempotency_key: string;
  dependencies: string[];
  rationale: string;
  action_type: 'execute_skill' | 'clarify_user';
  status: 'pending' | 'blocked';
}

export interface PlannerProposal {
  planner_provider: PlannerProvider;
  planner_model: string;
  planner_mode: 'deterministic' | 'model_augmented';
  confidence: number;
  requires_clarification: boolean;
  clarification_question?: string;
  actions: PlannedAction[];
  risk: {
    impact: string;
    rollback_plan: string;
  };
  requires_approval: boolean;
  reasoning: string[];
}

export interface VerificationResult {
  valid: boolean;
  issues: string[];
  warnings: string[];
}

export interface ProcessRequest extends IntentClassificationInput {
  skill_results?: SkillResult[];
}

export interface ProcessResponse {
  classification: IntentClassificationOutput;
  disambiguation: DisambiguationResponse;
  decomposition: TaskDecompositionOutput;
  plan: PlannerProposal;
  verification: VerificationResult;
  aggregation?: AggregationResponse;
  execution_status: ExecutionStatus;
}

export interface NormalizedReasoningRequest extends ProcessRequest {
  channel?: Channel;
  deployment_mode?: DeploymentMode;
  user_tier?: UserTier;
  user_profile: UserProfile;
  user_preferences: UserPreferences;
}
