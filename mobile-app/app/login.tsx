import { useState } from "react";
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useAuth } from "../src/auth/AuthContext";
import { Button, TextField } from "../src/components/ui";
import { colors, space } from "../src/theme";

export default function LoginScreen() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit() {
    setBusy(true);
    setError("");
    try {
      await login(username, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not sign in.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <SafeAreaView style={styles.safe}>
      <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1 }}>
        <ScrollView contentContainerStyle={styles.body} keyboardShouldPersistTaps="handled">
          <Text style={styles.kicker}>MyanKafe</Text>
          <Text style={styles.title}>Finance</Text>
          <Text style={styles.sub}>Sign in with your Finance username.</Text>
          <TextField label="Username" value={username} onChangeText={setUsername} placeholder="username" />
          <TextField label="Password" value={password} onChangeText={setPassword} secure={!show} />
          <Pressable onPress={() => setShow((v) => !v)} style={styles.toggle}>
            <Text style={styles.toggleText}>{show ? "Hide password" : "Show password"}</Text>
          </Pressable>
          {error ? <Text style={styles.error}>{error}</Text> : null}
          <Button label={busy ? "Signing in…" : "Sign in"} onPress={() => void submit()} disabled={busy || !username || !password} />
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.paper },
  body: { padding: space.lg, gap: 14 },
  kicker: { color: colors.accent, fontWeight: "800", letterSpacing: 1 },
  title: { fontSize: 36, fontWeight: "800", color: colors.ink },
  sub: { color: colors.muted, marginBottom: 8 },
  toggle: { minHeight: 44, justifyContent: "center" },
  toggleText: { color: colors.accent, fontWeight: "700" },
  error: { color: colors.negative, fontWeight: "600" },
});
