import { useQuery } from "@tanstack/react-query";
import { Stack, useLocalSearchParams } from "expo-router";
import { ScrollView, Text } from "react-native";
import { api } from "../../../src/api/runtime";
import { useEntity } from "../../../src/auth/EntityContext";
import { Card, EmptyState, ErrorState, LoadingState } from "../../../src/components/ui";
import { colors, space } from "../../../src/theme";

type Line = { date?: string; description?: string; debit?: string; credit?: string; balance?: string; running_balance?: string };

export default function LedgerScreen() {
  const { id, from, to } = useLocalSearchParams<{ id: string; from?: string; to?: string }>();
  const { entity } = useEntity();
  const query = useQuery({
    queryKey: ["ledger", entity?.PublicID, id, from, to],
    enabled: Boolean(entity && id),
    queryFn: () => api().request<{ items: Line[] }>(`/entities/${entity!.PublicID}/reports/account-ledger?account_id=${id}&from=${from || ""}&to=${to || ""}&limit=200`),
  });
  const items = query.data?.items ?? [];
  return (
    <ScrollView contentContainerStyle={{ padding: space.md, gap: 10, backgroundColor: colors.paper }}>
      <Stack.Screen options={{ title: "Account ledger" }} />
      <Text style={{ color: colors.muted }}>Read only</Text>
      {query.isLoading ? <LoadingState /> : query.isError ? <ErrorState message="Could not load the ledger." /> : items.length === 0 ? <EmptyState message="No ledger lines in this range." /> : items.map((line, index) => (
        <Card key={index}>
          <Text>{line.date}</Text>
          <Text>{line.description}</Text>
          <Text>Debit {line.debit || "0"}</Text>
          <Text>Credit {line.credit || "0"}</Text>
          {line.balance || line.running_balance ? <Text>Balance {line.balance || line.running_balance}</Text> : null}
        </Card>
      ))}
    </ScrollView>
  );
}
