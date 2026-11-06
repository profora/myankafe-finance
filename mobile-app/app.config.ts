import { ExpoConfig, ConfigContext } from "expo/config";

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { PRODUCTION_API_BASE_URL, assertProductionApiBaseUrl } = require("./productionApiGuard.js");

/**
 * MyanKafe Finance — native app.
 *
 * This is not the Staff app (com.myankafe.staff). Do not reuse the Staff EAS project.
 * No Finance EAS project ID is committed. Create one with:
 *   cd mobile-app && npx eas init
 * then set extra.eas.projectId locally or via EAS. Do not invent an ID here.
 */

type AppEnv = "development" | "preview" | "production";

function resolveAppEnv(): AppEnv {
  const raw = (process.env.EXPO_PUBLIC_APP_ENV || process.env.APP_ENV || "").trim().toLowerCase();
  if (raw === "production" || raw === "preview" || raw === "development") return raw;
  return "development";
}

function resolveApiBaseUrl(appEnv: AppEnv): string {
  const fromEnv = process.env.EXPO_PUBLIC_API_BASE_URL?.trim().replace(/\/$/, "");
  if (appEnv === "production") {
    return assertProductionApiBaseUrl(fromEnv || PRODUCTION_API_BASE_URL);
  }
  if (fromEnv) return fromEnv;
  return "http://127.0.0.1:8180/api/v1";
}

export default ({ config }: ConfigContext): ExpoConfig => {
  const appEnv = resolveAppEnv();
  const apiBaseUrl = resolveApiBaseUrl(appEnv);
  return {
    ...config,
    name: "MyanKafe Finance",
    slug: "myankafe-finance",
    scheme: "myankafefinance",
    version: "0.1.0",
    orientation: "portrait",
    icon: "./assets/icon.png",
    userInterfaceStyle: "light",
    splash: {
      image: "./assets/icon.png",
      resizeMode: "contain",
      backgroundColor: "#0B1F33",
    },
    ios: {
      supportsTablet: true,
      bundleIdentifier: "com.myankafe.finance",
    },
    android: {
      package: "com.myankafe.finance",
      adaptiveIcon: {
        foregroundImage: "./assets/icon.png",
        backgroundColor: "#0B1F33",
      },
    },
    plugins: ["expo-router", "expo-secure-store"],
    experiments: { typedRoutes: false },
    extra: {
      appEnv,
      apiBaseUrl,
    },
  };
};
