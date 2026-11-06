import { ExpoConfig, ConfigContext } from "expo/config";

// eslint-disable-next-line @typescript-eslint/no-require-imports
const { PRODUCTION_API_BASE_URL, resolveApiBaseUrl } = require("./productionApiGuard.js");

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

function apiBaseFor(appEnv: AppEnv): string {
  const fromEnv = process.env.EXPO_PUBLIC_API_BASE_URL?.trim();
  if (appEnv === "production") {
    return resolveApiBaseUrl(appEnv, fromEnv || PRODUCTION_API_BASE_URL);
  }
  return resolveApiBaseUrl(appEnv, fromEnv);
}

export default ({ config }: ConfigContext): ExpoConfig => {
  const appEnv = resolveAppEnv();
  const apiBaseUrl = apiBaseFor(appEnv);
  return {
    ...config,
    name: "MyanKafe Finance",
    slug: "myankafe-finance",
    scheme: "myankafefinance",
    version: "0.1.0",
    orientation: "portrait",
    icon: "./assets/icon.png",
    userInterfaceStyle: "light",
    backgroundColor: "#0B1F33",
    ios: {
      supportsTablet: true,
      bundleIdentifier: "com.myankafe.finance",
    },
    android: {
      package: "com.myankafe.finance",
      softwareKeyboardLayoutMode: "resize",
      blockedPermissions: [
        "android.permission.RECORD_AUDIO",
        "android.permission.SYSTEM_ALERT_WINDOW",
      ],
      adaptiveIcon: {
        foregroundImage: "./assets/icon.png",
        backgroundColor: "#0B1F33",
      },
    },
    plugins: [
      "expo-router",
      "expo-secure-store",
      "expo-font",
      [
        "expo-splash-screen",
        {
          image: "./assets/icon.png",
          backgroundColor: "#0B1F33",
          resizeMode: "contain",
        },
      ],
      [
        "expo-image-picker",
        {
          photosPermission: "MyanKafe Finance attaches receipt photos to draft transactions.",
          cameraPermission: "MyanKafe Finance uses the camera to capture receipts.",
        },
      ],
    ],
    experiments: { typedRoutes: false },
    extra: {
      appEnv,
      apiBaseUrl,
    },
  };
};
