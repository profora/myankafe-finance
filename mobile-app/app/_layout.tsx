import "react-native-gesture-handler";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Stack, useRouter, useSegments } from "expo-router";
import { useEffect } from "react";
import { ActivityIndicator, View } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { AuthProvider, useAuth } from "../src/auth/AuthContext";
import { EntityProvider } from "../src/auth/EntityContext";
import { colors } from "../src/theme";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 15_000 } },
});

function Gate() {
  const { ready, user } = useAuth();
  const segments = useSegments();
  const router = useRouter();
  useEffect(() => {
    if (!ready) return;
    const onLogin = segments[0] === "login";
    if (!user && !onLogin) router.replace("/login");
    if (user && onLogin) router.replace("/");
  }, [ready, user, segments, router]);
  if (!ready) {
    return (
      <View style={{ flex: 1, justifyContent: "center", backgroundColor: colors.paper }}>
        <ActivityIndicator color={colors.accent} />
      </View>
    );
  }
  return <Stack screenOptions={{ headerStyle: { backgroundColor: colors.paper }, headerTintColor: colors.ink, headerTitleStyle: { fontWeight: "700" } }} />;
}

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <EntityProvider>
            <Gate />
          </EntityProvider>
        </AuthProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}
