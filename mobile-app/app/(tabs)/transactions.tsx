import { FlashList } from "@shopify/flash-list";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { EntitySwitcher } from "../../src/components/EntitySwitcher";
import { NetworkBanner, useOffline } from "../../src/components/NetworkBanner";
import { EmptyState, ErrorState, LoadingState, Page, SelectField, StatusBadge, TextField } from "../../src/components/ui";
import { formatDisplay } from "../../src/lib/money";
import { canOperateLedger } from "../../src/lib/roles";
import { mergeTransactionRows, nextTransactionOffset, TRANSACTION_STATUSES, transactionQuery } from "../../src/lib/transactions";
import { colors, space } from "../../src/theme";

type Item = {
  row_id?: string;
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
type PageData = { items: Item[]; has_more: boolean };
type Fin = { PublicID: string; Name: string; Currency: string; Active: boolean };

const PAGE_SIZE = 40;

export default function TransactionsScreen() {
  const { entity } = useEntity();
  const router = useRouter();
  const offline = useOffline();
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [status, setStatus] = useState("");
  const [type, setType] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [accountId, setAccountId] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(q.trim()), 300);
    return () => clearTimeout(timer);
  }, [q]);

  useEffect(() => {
    setAccountId("");
    setQ("");
    setDebounced("");
  }, [entity?.PublicID]);

  const accounts = useQuery({
    queryKey: ["fins", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Fin[] }>(`/entities/${entity!.PublicID}/financial-accounts`),
  });

  const query = useInfiniteQuery({
    queryKey: ["transactions", entity?.PublicID, debounced, status, type, from, to, accountId],
    enabled: Boolean(entity),
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      api().request<PageData>(
        `/entities/${entity!.PublicID}/transactions?${transactionQuery({
          q: debounced,
          status,
          type,
          from,
          to,
          financialAccountId: accountId,
          offset: pageParam,
          limit: PAGE_SIZE,
        })}`,
      ),
    getNextPageParam: (last, _pages, lastParam) => nextTransactionOffset(last.items?.length ?? 0, Boolean(last.has_more), lastParam),
  });

  const items = mergeTransactionRows(
    (query.data?.pages ?? []).map((page) => page.items ?? []),
    (item) => item.row_id || `${item.id}:${item.financial_account_name || ""}:${item.signed_movement || ""}`,
  );

  // Keep filters/search outside FlashList so the Search TextInput is not remounted
  // when query results change mid-keystroke (which truncates Maestro inputText).
  const filters = (
    <Page>
      <EntitySwitcher />
      <Text style={styles.meta}>Status</Text>
      <View style={styles.row}>
        {["", ...TRANSACTION_STATUSES].map((value) => (
          <Pressable
            key={value || "all"}
            accessibilityRole="button"
            accessibilityLabel={value ? `Status ${value}` : "Any status"}
            testID={value ? `Status ${value}` : "Any status"}
            accessibilityState={{ selected: status === value }}
            onPress={() => setStatus(value)}
            style={[styles.chip, status === value && styles.chipOn]}
          >
            <Text style={status === value ? styles.chipOnText : styles.chipText}>{value || "Any"}</Text>
          </Pressable>
        ))}
      </View>
      <Text style={styles.meta}>Entry type</Text>
      <View style={styles.row}>
        {["", "INCOME", "EXPENSE", "ACCOUNT_TRANSFER"].map((value) => (
          <Pressable
            key={value || "any-type"}
            accessibilityRole="button"
            accessibilityLabel={value ? `Filter ${value}` : "Any entry type"}
            testID={value ? `Filter ${value}` : "Any entry type"}
            accessibilityState={{ selected: type === value }}
            onPress={() => setType(value)}
            style={[styles.chip, type === value && styles.chipOn]}
          >
            <Text style={type === value ? styles.chipOnText : styles.chipText}>{value || "Any"}</Text>
          </Pressable>
        ))}
      </View>
      <TextField label="Search" value={q} onChangeText={setQ} placeholder="Find by text or contact" />
      <View style={styles.dates}>
        <View style={{ flex: 1 }}><TextField label="From" value={from} onChangeText={setFrom} placeholder="YYYY-MM-DD" /></View>
        <View style={{ flex: 1 }}><TextField label="To" value={to} onChangeText={setTo} placeholder="YYYY-MM-DD" /></View>
      </View>
      <SelectField
        label="Financial account"
        value={accountId}
        onChange={setAccountId}
        options={[{ id: "", label: "Any account" }, ...(accounts.data?.items ?? []).filter((item) => item.Active).map((item) => ({ id: item.PublicID, label: `${item.Name} (${item.Currency})` }))]}
      />
      {canOperateLedger(entity?.Role) && !offline ? (
        <Pressable accessibilityRole="button" accessibilityLabel="New entry" testID="New entry" style={styles.new} onPress={() => router.push("/transaction/new")}>
          <Text style={styles.newText}>+ New Entry</Text>
        </Pressable>
      ) : null}
    </Page>
  );

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.paper }}>
      <NetworkBanner />
      <View style={{ flex: 1 }}>
        {filters}
        {query.isLoading ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState message="Could not load transactions." onRetry={() => void query.refetch()} />
        ) : (
          <FlashList
            data={items}
            ListEmptyComponent={<EmptyState message="No transactions match these filters." />}
            onEndReached={() => {
              if (query.hasNextPage && !query.isFetchingNextPage) void query.fetchNextPage();
            }}
            renderItem={({ item }) => (
              <Pressable accessibilityRole="button" accessibilityLabel={`${item.status} ${item.type} ${item.description}`} style={styles.card} onPress={() => router.push(`/transaction/${item.id}`)}>
                <View style={styles.between}>
                  <Text style={styles.date}>{item.date}</Text>
                  <StatusBadge status={item.status} />
                </View>
                <Text style={styles.desc}>{item.description || item.type}</Text>
                <Text style={styles.meta}>{item.type} · {item.financial_account_name || item.contact_name || "No counterparty"}</Text>
                <Text style={styles.amount}>{formatDisplay(item.signed_movement || "0", item.account_currency || entity?.FunctionalCurrency || "")}</Text>
              </Pressable>
            )}
          />
        )}
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  dates: { flexDirection: "row", gap: 8 },
  chip: { minHeight: 44, justifyContent: "center", paddingHorizontal: 12, borderRadius: 999, borderWidth: 1, borderColor: colors.line, backgroundColor: "#fff" },
  chipOn: { backgroundColor: colors.ink, borderColor: colors.ink },
  chipText: { color: colors.ink },
  chipOnText: { color: "#fff", fontWeight: "700" },
  meta: { color: colors.muted },
  new: { minHeight: 48, borderRadius: 12, backgroundColor: colors.accent, alignItems: "center", justifyContent: "center" },
  newText: { color: "#fff", fontWeight: "800" },
  card: { marginHorizontal: space.md, marginBottom: 10, padding: 12, borderRadius: 14, backgroundColor: "#fff", borderWidth: 1, borderColor: colors.line, gap: 4 },
  between: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  date: { color: colors.muted, flexShrink: 1 },
  desc: { fontWeight: "700", color: colors.ink, fontSize: 16 },
  amount: { fontVariant: ["tabular-nums"], fontWeight: "800", color: colors.ink, fontSize: 18 },
});
