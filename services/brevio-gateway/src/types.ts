export interface GatewayConfig {
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
