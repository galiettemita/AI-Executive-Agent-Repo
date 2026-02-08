import AsyncStorage from "@react-native-async-storage/async-storage";

const TOKEN_KEY = "eai_token";
const USER_KEY = "eai_user_id";

export async function saveSession(token: string, userId: string) {
  await AsyncStorage.multiSet([
    [TOKEN_KEY, token],
    [USER_KEY, userId],
  ]);
}

export async function clearSession() {
  await AsyncStorage.multiRemove([TOKEN_KEY, USER_KEY]);
}

export async function loadSession() {
  const entries = await AsyncStorage.multiGet([TOKEN_KEY, USER_KEY]);
  const data: Record<string, string | null> = {};
  for (const [key, value] of entries) {
    data[key] = value;
  }
  return {
    token: data[TOKEN_KEY] || null,
    userId: data[USER_KEY] || null,
  };
}
