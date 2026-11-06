import Constants from "expo-constants";
import { createClient } from "./client";
import { clearSession, getSessionToken } from "../auth/session";

type Extra = { apiBaseUrl?: string; appEnv?: string };

export function appExtra(): Extra {
  return (Constants.expoConfig?.extra as Extra | undefined) ?? {};
}

export function apiHostLabel(): string {
  try {
    return new URL(appExtra().apiBaseUrl || "").hostname;
  } catch {
    return "";
  }
}

let unauthorized: () => void = () => undefined;

export function setUnauthorizedHandler(handler: () => void) {
  unauthorized = handler;
}

export function api() {
  const base = appExtra().apiBaseUrl || "";
  return createClient({
    baseUrl: base,
    getToken: getSessionToken,
    onUnauthorized: async () => {
      await clearSession();
      unauthorized();
    },
  });
}
