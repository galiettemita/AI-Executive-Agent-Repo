import { apiFetch } from "./client";

export type PairResponse = {
  access_token: string;
  token_type: string;
  expires_at: string;
  user_id: string;
};

export async function pairDevice(code: string) {
  return apiFetch<PairResponse>("/auth/pair", {
    method: "POST",
    body: { code },
  });
}
