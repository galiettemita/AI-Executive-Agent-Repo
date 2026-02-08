import { apiFetch } from "./client";

export async function runEmailMonitoring(userId: string, token: string) {
  return apiFetch<{ ok: boolean; result: any }>("/email/intelligence/monitoring/run", {
    method: "POST",
    token,
    body: { user_id: userId },
  });
}
