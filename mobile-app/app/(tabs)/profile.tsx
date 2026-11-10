import Constants from "expo-constants";
import { useRouter } from "expo-router";
import { Alert, ScrollView, StyleSheet, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { apiHostLabel, appExtra } from "../../src/api/runtime";
import { useAuth } from "../../src/auth/AuthContext";
import { useEntity } from "../../src/auth/EntityContext";
import { EntitySwitcher } from "../../src/components/EntitySwitcher";
import { Button, Card } from "../../src/components/ui";
import { colors, space } from "../../src/theme";

export default function ProfileScreen() {
  const { user, logout } = useAuth();
  const { entity } = useEntity();
  const router = useRouter();
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.paper }}>
      <ScrollView contentContainerStyle={styles.body}>
        <Text style={styles.title}>Profile</Text>
        <Card>
          <Text style={styles.name}>{user?.display_name}</Text>
          <Text style={styles.meta}>Username {user?.username}</Text>
          <Text style={styles.meta}>Entity {entity?.Name || "—"}</Text>
          <Text style={styles.meta}>Role {entity?.Role || "—"}</Text>
          <Text style={styles.meta}>App {Constants.expoConfig?.version}</Text>
          <Text style={styles.meta}>Environment {appExtra().appEnv}</Text>
          <Text style={styles.meta}>API host {apiHostLabel()}</Text>
        </Card>
        <EntitySwitcher />
        <Button label="Change password" tone="secondary" onPress={() => router.push("/change-password")} />
        <Button label="Active sessions" tone="secondary" onPress={() => router.push("/sessions")} />
        <Button
          label="Sign out"
          tone="danger"
          onPress={() =>
            Alert.alert("Sign out", "Sign out of MyanKafe Finance on this device?", [
              { text: "Cancel", style: "cancel" },
              { text: "Sign out", style: "destructive", onPress: () => void logout() },
            ])
          }
        />
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12 },
  title: { fontSize: 28, fontWeight: "800", color: colors.ink },
  name: { fontSize: 20, fontWeight: "800", color: colors.ink },
  meta: { color: colors.muted },
});
