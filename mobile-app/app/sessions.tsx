import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Stack } from "expo-router";
import { Alert, ScrollView, Text } from "react-native";
import { api } from "../src/api/runtime";
import { Button, Card, ErrorState, LoadingState } from "../src/components/ui";
import { colors, space } from "../src/theme";

type Session = { id: string; user_agent: string; ip_address: string; created_at: string; last_seen_at: string; expires_at: string; current: boolean };

export default function SessionsScreen() {
  const client = useQueryClient();
  const query = useQuery({
    queryKey: ["sessions"],
    queryFn: () => api().request<{ items: Session[] }>("/auth/sessions"),
  });
  const revoke = useMutation({
    mutationFn: (id: string) => api().request(`/auth/sessions/${id}`, { method: "DELETE" }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["sessions"] }),
  });
  const revokeOthers = useMutation({
    mutationFn: () => api().request("/auth/sessions/revoke-others", { method: "POST", body: "{}" }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["sessions"] }),
  });
  return (
    <ScrollView contentContainerStyle={{ padding: space.md, gap: 12, backgroundColor: colors.paper }}>
      <Stack.Screen options={{ title: "Active sessions" }} />
      {query.isLoading ? <LoadingState /> : query.isError ? <ErrorState message="Could not load sessions." /> : (query.data?.items ?? []).map((session) => (
        <Card key={session.id}>
          <Text style={{ fontWeight: "700" }}>{session.current ? "This device" : "Other session"}</Text>
          <Text>{session.user_agent}</Text>
          <Text>IP {session.ip_address}</Text>
          <Text>Created {session.created_at}</Text>
          <Text>Last seen {session.last_seen_at}</Text>
          <Text>Expires {session.expires_at}</Text>
          {!session.current ? <Button label="Revoke" tone="danger" onPress={() => Alert.alert("Revoke session", "Sign that device out?", [{ text: "Cancel", style: "cancel" }, { text: "Revoke", style: "destructive", onPress: () => revoke.mutate(session.id) }])} /> : null}
        </Card>
      ))}
      <Button
        label="Revoke all other sessions"
        tone="danger"
        onPress={() => Alert.alert("Revoke other sessions", "This signs out every device except this one.", [{ text: "Cancel", style: "cancel" }, { text: "Revoke others", style: "destructive", onPress: () => revokeOthers.mutate() }])}
      />
    </ScrollView>
  );
}
