import { useQuery } from "@tanstack/react-query";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { EntitySwitcher } from "../../src/components/EntitySwitcher";
import { NetworkBanner, useOffline } from "../../src/components/NetworkBanner";
import { Card, EmptyState, ErrorState, LoadingState } from "../../src/components/ui";
import { fiscalYearLabel } from "../../src/lib/dates";
import { formatDisplay } from "../../src/lib/money";
import { canOperateLedger } from "../../src/lib/roles";
import { colors, space } from "../../src/theme";

type Dash = { income: string; expenses: string; net_profit: string };
type Balance = { entity_id: string; entity_name: string; id: string; name: string; kind: string; currency: string; balance: string; ledger_account_id?: string };
type Combined = { reporting_currency: string; income: string; expenses: string; net_profit: string; entities: { entity_name: string; functional_currency: string; net_profit: string }[] };

const KINDS = ["CASH", "BANK", "MOBILE_WALLET", "CREDIT_CARD", "OTHER"] as const;

export default function HomeScreen() {
  const { entity, entities, loading } = useEntity();
  const router = useRouter();
  const offline = useOffline();
  const [combined, setCombined] = useState(false);
  const dash = useQuery({
    queryKey: ["dashboard", entity?.PublicID],
    enabled: Boolean(entity) && !combined,
    staleTime: 20_000,
    queryFn: () => api().request<Dash>(`/entities/${entity!.PublicID}/dashboard`),
  });
  const balances = useQuery({
    queryKey: ["balances", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Balance[] }>("/financial-account-balances"),
  });
  const all = useQuery({
    queryKey: ["combined", entity?.FunctionalCurrency],
    enabled: combined && entities.length > 1 && Boolean(entity),
    queryFn: () => api().request<Combined>(`/dashboard/combined?currency=${encodeURIComponent(entity!.FunctionalCurrency)}`),
  });
  const mayEdit = canOperateLedger(entity?.Role) && !offline;

  if (loading || !entity) return <LoadingState label="Loading entities" />;

  const mine = (balances.data?.items ?? []).filter((item) => item.entity_id === entity.PublicID);
  const grouped = KINDS.map((kind) => ({ kind, items: mine.filter((item) => item.kind === kind) })).filter((g) => g.items.length);

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.paper }}>
      <NetworkBanner />
      <ScrollView contentContainerStyle={styles.body}>
        <Text style={styles.brand}>MyanKafe Finance</Text>
        <EntitySwitcher />
        {entities.length > 1 ? (
          <Pressable onPress={() => setCombined((v) => !v)} style={styles.link}>
            <Text style={styles.linkText}>{combined ? "Show this entity" : "All entities overview"}</Text>
          </Pressable>
        ) : null}
        {combined && all.data ? (
          <Card>
            <Text style={styles.section}>All entities</Text>
            <Text style={styles.meta}>Reporting currency {all.data.reporting_currency}. This does not replace entity books.</Text>
            <Kpi label="Income" value={formatDisplay(all.data.income, all.data.reporting_currency)} />
            <Kpi label="Expenses" value={formatDisplay(all.data.expenses, all.data.reporting_currency)} />
            <Kpi label="Net Profit" value={formatDisplay(all.data.net_profit, all.data.reporting_currency)} />
          </Card>
        ) : dash.isLoading ? (
          <LoadingState />
        ) : dash.isError ? (
          <ErrorState message="Could not load the dashboard." onRetry={() => void dash.refetch()} />
        ) : dash.data ? (
          <Card>
            <Kpi label="Income" value={formatDisplay(dash.data.income, entity.FunctionalCurrency)} />
            <Kpi label="Expenses" value={formatDisplay(dash.data.expenses, entity.FunctionalCurrency)} />
            <Kpi label="Net Profit" value={formatDisplay(dash.data.net_profit, entity.FunctionalCurrency)} />
            <Kpi label="Fiscal Year" value={fiscalYearLabel(entity.FiscalMonth, entity.FiscalDay, entity.Timezone)} />
          </Card>
        ) : null}
        <Text style={styles.section}>Accounts</Text>
        {grouped.length === 0 ? <EmptyState message="No financial accounts for this entity." /> : null}
        {grouped.map((group) => (
          <View key={group.kind} style={{ gap: 8 }}>
            <Text style={styles.kind}>{group.kind.replace(/_/g, " ")}</Text>
            {group.items.map((item) => (
              <Pressable key={item.id} accessibilityRole="button" accessibilityLabel={`${item.name} ${formatDisplay(item.balance, item.currency)}`} onPress={() => item.ledger_account_id && router.push({ pathname: "/account/[id]/ledger", params: { id: item.ledger_account_id } })}>
                <Card>
                  <Text style={styles.account}>{item.name}</Text>
                  <Text style={styles.meta}>
                    {item.kind.replace(/_/g, " ")} · {item.entity_name} · {item.currency}
                  </Text>
                  <Text style={styles.amount}>{formatDisplay(item.balance, item.currency)}</Text>
                </Card>
              </Pressable>
            ))}
          </View>
        ))}
        <Text style={styles.section}>Quick actions</Text>
        {mayEdit ? (
          <>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="+ New Income"
              testID="+ New Income"
              style={styles.primary}
              onPress={() => router.push({ pathname: "/transaction/new", params: { type: "INCOME" } })}
            >
              <Text style={styles.primaryText}>+ New Income</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="New Expense"
              testID="New Expense"
              style={styles.secondary}
              onPress={() => router.push({ pathname: "/transaction/new", params: { type: "EXPENSE" } })}
            >
              <Text style={styles.secondaryText}>New Expense</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Transfer"
              testID="Transfer"
              style={styles.secondary}
              onPress={() => router.push("/transfer/new")}
            >
              <Text style={styles.secondaryText}>Transfer</Text>
            </Pressable>
          </>
        ) : (
          <Text style={styles.meta}>{offline ? "Reconnect to record entries." : "This role can review but not post entries."}</Text>
        )}
        <Pressable style={styles.secondary} onPress={() => router.push("/transactions")}>
          <Text style={styles.secondaryText}>Transactions</Text>
        </Pressable>
        <Pressable style={styles.secondary} onPress={() => router.push("/reports")}>
          <Text style={styles.secondaryText}>Reports</Text>
        </Pressable>
      </ScrollView>
    </SafeAreaView>
  );
}

function Kpi({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.kpi}>
      <Text style={styles.meta}>{label}</Text>
      <Text style={styles.amount}>{value}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12 },
  brand: { color: colors.accent, fontWeight: "800" },
  section: { marginTop: 8, fontSize: 18, fontWeight: "800", color: colors.ink },
  kind: { color: colors.muted, fontWeight: "700" },
  account: { fontWeight: "700", color: colors.ink },
  meta: { color: colors.muted },
  amount: { fontSize: 22, fontWeight: "800", color: colors.ink, fontVariant: ["tabular-nums"] },
  kpi: { paddingVertical: 6 },
  link: { minHeight: 44, justifyContent: "center" },
  linkText: { color: colors.accent, fontWeight: "700" },
  primary: { minHeight: 52, borderRadius: 14, backgroundColor: colors.accent, alignItems: "center", justifyContent: "center" },
  primaryText: { color: "#fff", fontWeight: "800", fontSize: 17 },
  secondary: { minHeight: 48, borderRadius: 12, borderWidth: 1, borderColor: colors.line, alignItems: "center", justifyContent: "center", backgroundColor: "#fff" },
  secondaryText: { color: colors.ink, fontWeight: "700" },
});
