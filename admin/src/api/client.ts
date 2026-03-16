const API_BASE = '';

async function apiRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!response.ok) {
    throw new Error(`API error ${response.status}: ${response.statusText}`);
  }
  return response.json() as Promise<T>;
}

export interface ServiceHealth {
  status: string;
  version: string;
  service: string;
  uptime_ms: number;
  checks?: Record<string, string>;
}

export interface CapabilityRecommendation {
  id: string;
  capability_key: string;
  confidence: number;
  reason: string;
}

export interface IngestResult {
  message_id: string;
  status: string;
  workflow_id?: string;
}

export const apiGet = <T>(path: string): Promise<T> =>
  apiRequest<T>(path);

export const fetchBrainHealth = (): Promise<ServiceHealth> =>
  apiRequest<ServiceHealth>('http://localhost:18081/health/deep');

export const fetchControlHealth = (): Promise<ServiceHealth> =>
  apiRequest<ServiceHealth>('http://localhost:18082/health/deep');

export const fetchCapabilityRecommendations = (workspaceId: string): Promise<CapabilityRecommendation[]> =>
  apiRequest<CapabilityRecommendation[]>(
    `http://localhost:18082/v1/capabilities/recommendations?workspace_id=${encodeURIComponent(workspaceId)}`
  );

export const ingestTestMessage = (workspaceId: string, content: string): Promise<IngestResult> =>
  apiRequest<IngestResult>('http://localhost:18081/v1/brain/ingest', {
    method: 'POST',
    body: JSON.stringify({ id: `admin-${Date.now()}`, workspace_id: workspaceId, content, channel: 'admin_test' }),
  });

export const fetchSelfModPolicy = (workspaceId: string) =>
  apiRequest(`http://localhost:18082/v1/self-modification/policy/${encodeURIComponent(workspaceId)}`);
