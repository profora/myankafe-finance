import { FlashList } from "@shopify/flash-list";
import { useQuery } from "@tanstack/react-query";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { EntitySwitcher } from "../../src/components/EntitySwitcher";
import { NetworkBanner, useOffline } from "../../src/components/NetworkBanner";
import { EmptyState, ErrorState, LoadingState, StatusBadge, TextField } from "../../src/components/ui";
import { formatDisplay } from "../../src/lib/money";
import { canOperateLedger } from "../../src/lib/roles";
import { transactionQuery, TRANSACTION_STATUSES, TRANSACTION_TYPES } from "../../src/lib/transactions";
import { colors, space } from "../../src/theme";

type Item = {
  id: string;
  date: string;
  description: string;
  type: string;
  status: string;
  signed_movement?: string | null;
  account_currency?: string | null;
  financial_account_name?: string | null;
  contact_name?: string | null;
};
type Page = { items: Item[]; has_more: boolean };

export default function TransactionsScreen() {
  const { entity } = useEntity();
  const router = useRouter();
  const offline = useOffline();
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [type, setType] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [offset, setOffset] = useState(0);
  const query = useQuery({
    queryKey: ["transactions", entity?.PublicID, q, status, type, from, to, offset],
    enabled: Boolean(entity),
    queryFn: () =>
      api().request<Page>(
        `/entities/${entity!.PublicID}/transactions?${transactionQuery({ q, status, type, from, to, offset, limit: 40 })}`,
      ),
  });
  const items = query.data?.items ?? [];
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.paper }}>
      <NetworkBanner />
      <View style={styles.head}>
        <EntitySwitcher />
        <TextField label="Search" value={q} onChangeText={(v) => { setOffset(0); setQ(v); }} placeholder="Description or contact" />
        <Text style={styles.meta}>Status</Text>
        <View style={styles.row}>
          {["", ...TRANSACTION_STATUSES].map((value) => (
            <Pressable key={value || "all"} onPress={() => { setOffset(0); setStatus(value); }} style={[styles.chip, status === value && styles.chipOn]}>
              <Text style={status === value ? styles.chipOnText : styles.chipText}>{value || "Any"}</Text>
            </Pressable>
          ))}
        </View>
        <Text style={styles.meta}>Type</Text>
        <View style={styles.row}>
          {["", "INCOME", "EXPENSE", "ACCOUNT_TRANSFER"].map((value) => (
            <Pressable key={value || "any-type"} onPress={() => { setOffset(0); setType(value); }} style={[styles.chip, type === value && styles.chipOn]}>
              <Text style={type === value ? styles.chipOnText : styles.chipText}>{value || "Any"}</Text>
            </Pressable>
          ))}
        </View>
        <View style={styles.dates}>
          <View style={{ flex: 1 }}><TextField label="From" value={from} onChangeText={setFrom} placeholder="YYYY-MM-DD" /></View>
          <View style={{ flex: 1 }}><TextField label="To" value={to} onChangeText={setTo} placeholder="YYYY-MM-DD" /></View>
        </View>
        {canOperateLedger(entity?.Role) && !offline ? (
          <Pressable style={styles.new} onPress={() => router.push("/transaction/new")}>
            <Text style={styles.newText}>+ New Entry</Text>
          </Pressable>
        ) : null}
      </View>
      {query.isLoading ? <LoadingState /> : query.isError ? <ErrorState message="Could not load transactions." onRetry={() => void query.refetch()} /> : (
        <FlashList
          data={items}
          estimatedItemSize={92}
          ListEmptyComponent={<EmptyState message="No transactions match these filters." />}
          onEndReached={() => {
            if (query.data?.has_more) setOffset((n) => n + 40);
          }}
          renderItem={({ item }) => (
            <Pressable style={styles.card} onPress={() => router.push(`/transaction/${item.id}`)}>
              <View style={styles.between}>
                <Text style={styles.date}>{item.date}</Text>
                <StatusBadge status={item.status} />
              </View>
              <Text style={styles.desc}>{item.description || item.type}</Text>
              <Text style={styles.meta}>{item.type} · {item.financial_account_name || item.contact_name || "No counterparty"}</Text>
              <Text style={styles.amount}>{formatDisplay(item.signed_movement || "0", item.account_currency || entity?.FunctionalCurrency || "")}</Text>
            </Pressable>
          )}
          ListFooterComponent={TRANSACTION_TYPES.includes(type as never) ? null : null}
        />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  head: { padding: space.md, gap: 8 },
  row: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  dates: { flexDirection: "row", gap: 8 },
  chip: { minHeight: 36, justifyContent: "center", paddingHorizontal: 10, borderRadius: 999, borderWidth: 1, borderColor: colors.line, backgroundColor: "#fff" },
  chipOn: { backgroundColor: colors.ink, borderColor: colors.ink },
  chipText: { color: colors.ink },
  chipOnText: { color: "#fff", fontWeight: "700" },
  meta: { color: colors.muted },
  new: { minHeight: 48, borderRadius: 12, backgroundColor: colors.accent, alignItems: "center", justifyContent: "center" },
  newText: { color: "#fff", fontWeight: "800" },
  card: { marginHorizontal: space.md, marginBottom: 10, padding: 12, borderRadius: 14, backgroundColor: "#fff", borderWidth: 1, borderColor: colors.line, gap: 4 },
  between: { flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  date: { color: colors.muted },
  desc: { fontWeight: "700", color: colors.ink, fontSize: 16 },
  amount: { fontVariant: ["tabular-nums"], fontWeight: "800", color: colors.ink, fontSize: 18 },
});
