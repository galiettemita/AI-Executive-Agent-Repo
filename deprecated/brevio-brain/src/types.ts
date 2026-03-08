export type Channel = 'WHATSAPP' | 'IMESSAGE' | 'API';

export type UserTier = 'free' | 'pro' | 'enterprise' | 'admin' | 'service';

export interface BrainConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
  disambiguationConfigPath: string;
}

export interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  userId?: string;
}

export interface IntentClassificationInput {
  message_text: string;
  user_profile?: {
    timezone?: string;
    locale?: string;
    enabled_skills?: string[];
    recent_intents?: string[];
    preferences?: UserPreferences;
  };
  context?: {
    time_of_day?: string;
    day_of_week?: string;
    active_session_minutes?: number;
  };
}

export interface IntentClassificationOutput {
  intent: string;
  confidence: number;
  skills: string[];
  requires_decomposition: boolean;
  reasoning: string;
}

export interface TaskDescriptor {
  id: string;
  skill_id: string;
  input: Record<string, unknown>;
  dependencies: string[];
  priority: number;
}

export interface TaskDecompositionOutput {
  tasks: TaskDescriptor[];
  execution_order: 'parallel' | 'sequential' | 'mixed';
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

export interface DisambiguationRequest {
  message_text: string;
  intent?: string;
  candidate_skills?: string[];
  deployment_mode?: 'cloud' | 'local_mac' | 'mcp';
  user_tier?: UserTier;
  user_preferences?: UserPreferences;
}

export interface DisambiguationResponse {
  resolved_skills: string[];
  group_hits: string[];
}

export interface SkillResult {
  skill_id: string;
  status: 'SUCCESS' | 'PARTIAL' | 'FAILED' | 'TIMEOUT';
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
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
}

export interface DisambiguationRule {
  group: string;
  values: Record<string, string | string[]>;
}
