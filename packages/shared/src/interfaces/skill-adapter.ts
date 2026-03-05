import type { JSONSchema7 } from 'json-schema';
import type { SkillResult } from '../schemas/skill-result.js';

export interface ISkillAdapter {
  readonly id: string;
  readonly plane: 'gateway' | 'brain' | 'hands';
  readonly requiredScopes: string[];
  readonly inputSchema: JSONSchema7;
  readonly outputSchema: JSONSchema7;

  execute(input: SkillInput, ctx: SkillContext): Promise<SkillResult>;
  healthCheck(): Promise<boolean>;
  undo?(input: SkillInput): Promise<void>;
}

export interface OAuthToken {
  accessToken: string;
  refreshToken?: string;
  expiresAt?: string;
}

export interface UserProfile {
  id: string;
  timezone: string;
  locale: string;
}

export interface StructuredLogger {
  info(payload: Record<string, unknown>, message?: string): void;
  warn(payload: Record<string, unknown>, message?: string): void;
  error(payload: Record<string, unknown>, message?: string): void;
}

export interface Tracer {
  startSpan(name: string): unknown;
}

export interface CacheClient {
  get(key: string): Promise<string | null>;
  set(key: string, value: string, ttlSeconds?: number): Promise<void>;
}

export interface MCPClient {
  call(tool: string, input: unknown): Promise<unknown>;
}

export interface SkillInput {
  [key: string]: unknown;
}

export interface SkillContext {
  userId: string;
  oauthTokens: Map<string, OAuthToken>;
  userProfile: UserProfile;
  logger: StructuredLogger;
  tracer: Tracer;
  cache: CacheClient;
  config: Record<string, unknown>;
  mcpClient?: MCPClient;
}
