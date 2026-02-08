import { apiFetch } from "./client";

export type AlertItem = {
  id: number;
  subject?: string;
  sender?: string;
  reason?: string;
  priority?: number;
  alert_channel?: string;
  created_at?: string;
};

export async function fetchEmailAlerts(userId: string, token: string) {
  return apiFetch<{ ok: boolean; alerts: AlertItem[] }>(
    `/email/intelligence/monitoring/alerts?user_id=${encodeURIComponent(userId)}`,
    { token }
  );
}

export async function fetchNotifications(userId: string, token: string) {
  return apiFetch<{ items: any[] }>(
    `/notifications?user_id=${encodeURIComponent(userId)}`,
    { token }
  );
}
