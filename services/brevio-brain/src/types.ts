export type ExternalModelEgress = 'allow' | 'redacted_only' | 'deny';

export interface BrainConfig {
  serviceName: string;
  version: string;
  environment: string;
  port: number;
  shutdownTimeoutMs: number;
}

export interface RequestContext {
  traceId: string;
  spanId: string;
  requestId: string;
  userId?: string;
}
