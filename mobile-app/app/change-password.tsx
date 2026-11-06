import { Stack, useRouter } from "expo-router";
import { useState } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, Text } from "react-native";
import { friendlyError } from "../src/api/errors";
import { api } from "../src/api/runtime";
import { useAuth } from "../src/auth/AuthContext";
import { Button, TextField } from "../src/components/ui";
import { passwordChangeError } from "../src/lib/password";
import { colors, space } from "../src/theme";

export default function ChangePasswordScreen() {
  const { replaceToken } = useAuth();
  const router = useRouter();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function submit() {
    const problem = passwordChangeError(current, next);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const result = await api().request<{ session_token?: string; session_expires_at?: string }>("/auth/change-password", {
        method: "POST",
        headers: { "X-Session-Transport": "bearer" },
        endSessionOn401: false,
        body: JSON.stringify({ current_password: current, new_password: next }),
      });
      if (!result.session_token) throw new Error("Password changed, but no replacement session was returned. Sign in again.");
      await replaceToken(result.session_token, result.session_expires_at);
      router.back();
    } catch (err) {
      setError(friendlyError(err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1, backgroundColor: colors.paper }}>
      <Stack.Screen options={{ title: "Change password" }} />
      <ScrollView contentContainerStyle={styles.body}>
        <TextField label="Current password" value={current} onChangeText={setCurrent} secure />
        <TextField label="New password" value={next} onChangeText={setNext} secure />
        {error ? <Text style={styles.error}>{error}</Text> : null}
        <Button label={busy ? "Saving…" : "Change password"} disabled={busy || !current || !next} onPress={() => void submit()} />
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12 },
  error: { color: colors.negative, fontWeight: "700" },
});
