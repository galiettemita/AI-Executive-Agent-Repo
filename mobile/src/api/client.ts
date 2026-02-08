import { Platform } from "react-native";

export const API_BASE_URL =
  process.env.EXPO_PUBLIC_API_BASE_URL ||
  (Platform.OS === "android" ? "http://10.0.2.2:8000" : "http://localhost:8000");

export type ApiResult<T> = {
  ok: boolean;
  data?: T;
  error?: string;
};

export async function apiFetch<T>(
  path: string,
  options: {
    method?: string;
    token?: string | null;
    body?: unknown;
    headers?: Record<string, string>;
  } = {}
): Promise<ApiResult<T>> {
  const { method = "GET", token, body, headers } = options;
  const url = `${API_BASE_URL}${path}`;

  try {
    const resp = await fetch(url, {
      method,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(headers || {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });

    const text = await resp.text();
    const payload = text ? JSON.parse(text) : null;

    if (!resp.ok) {
      return {
        ok: false,
        error: payload?.detail || payload?.message || "Request failed",
      };
    }

    return { ok: true, data: payload };
  } catch (err: any) {
    return { ok: false, error: err?.message || "Network error" };
  }
}
