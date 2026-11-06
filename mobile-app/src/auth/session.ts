import * as SecureStore from "expo-secure-store";

const TOKEN_KEY = "mk_finance_session_token";
const EXPIRES_KEY = "mk_finance_session_expires";

export async function saveSession(token: string, expiresAt?: string) {
  await SecureStore.setItemAsync(TOKEN_KEY, token);
  if (expiresAt) await SecureStore.setItemAsync(EXPIRES_KEY, expiresAt);
}

export async function getSessionToken(): Promise<string | null> {
  return SecureStore.getItemAsync(TOKEN_KEY);
}

export async function clearSession() {
  await SecureStore.deleteItemAsync(TOKEN_KEY);
  await SecureStore.deleteItemAsync(EXPIRES_KEY);
}
