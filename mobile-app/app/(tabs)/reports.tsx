import { useQuery } from "@tanstack/react-query";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { EntitySwitcher } from "../../src/components/EntitySwitcher";
import { NetworkBanner } from "../../src/components/NetworkBanner";
import { Card, EmptyState, ErrorState, LoadingState, TextField } from "../../src/components/ui";
import { todayInTimeZone } from "../../src/lib/dates";
import { formatDisplay, subtractAmountStrings, sumAmountStrings } from "../../src/lib/money";
import { groupBalanceSheet, groupProfitLoss, reportRequest } from "../../src/lib/reports";
import { colors, space } from "../../src/theme";

const REPORTS = [
  ["profit-loss", "Profit & Loss"],
  ["balance-sheet", "Balance Sheet"],
  ["trial-balance", "Trial Balance"],
  ["cash-movement", "Cash Movement"],
  ["sales-analysis", "Sales Analysis"],
  ["general-ledger", "General Ledger"],
] as const;

type Row = Record<string, string | undefined>;

export default function ReportsScreen() {
  const { entity } = useEntity();
  const router = useRouter();
  const [report, setReport] = useState<(typeof REPORTS)[number][0]>("profit-loss");
  const [from, setFrom] = useState(entity ? todayInTimeZone(entity.Timezone).slice(0, 8) + "01" : "");
  const [to, setTo] = useState(entity ? todayInTimeZone(entity.Timezone) : "");
  const query = useQuery({
    queryKey: ["report", entity?.PublicID, report, from, to],
    enabled: Boolean(entity),
    queryFn: async () => {
      return api().request<unknown>(reportRequest(entity!.PublicID, report, from, to));
    },
  });
  const items = extractItems(query.data);
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.paper }}>
      <NetworkBanner />
      <ScrollView contentContainerStyle={styles.body}>
        <EntitySwitcher />
        <View style={styles.row}>
          <View style={{ flex: 1 }}><TextField label="From" value={from} onChangeText={setFrom} /></View>
          <View style={{ flex: 1 }}><TextField label="To / Through" value={to} onChangeText={setTo} /></View>
        </View>
        <View style={styles.row}>
          {REPORTS.map(([id, label]) => (
            <Pressable key={id} onPress={() => setReport(id)} style={[styles.chip, report === id && styles.chipOn]}>
              <Text style={report === id ? styles.on : styles.off}>{label}</Text>
            </Pressable>
          ))}
        </View>
        {query.isLoading ? <LoadingState /> : query.isError ? <ErrorState message="Could not load this report." onRetry={() => void query.refetch()} /> : items.length === 0 && !earnings(query.data) ? <EmptyState message="No rows in this range." /> : (
          <ReportBody report={report} items={items} currency={entity?.FunctionalCurrency || ""} earnings={earnings(query.data)} onAccount={(id) => router.push({ pathname: "/account/[id]/ledger", params: { id, from, to } })} />
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

function extractItems(data: unknown): Row[] {
  if (!data) return [];
  if (Array.isArray(data)) return data as Row[];
  if (typeof data === "object" && data && "items" in data && Array.isArray((data as { items: unknown }).items)) {
    return (data as { items: Row[] }).items;
  }
  return [];
}

function earnings(data: unknown): string {
  if (data && typeof data === "object" && "current_earnings" in data) return String((data as { current_earnings?: string }).current_earnings || "");
  return "";
}

function ReportBody({ report, items, currency, earnings: currentEarnings, onAccount }: { report: string; items: Row[]; currency: string; earnings?: string; onAccount: (id: string) => void }) {
  if (report === "profit-loss") {
    const grouped = groupProfitLoss(items);
    const incomeTotal = sumAmountStrings(grouped.income.map((row) => row.amount || "0")) || "0";
    const expenseTotal = sumAmountStrings(grouped.expenses.map((row) => row.amount || "0")) || "0";
    const net = subtractAmountStrings(incomeTotal, expenseTotal) || "0";
    return (
      <View style={{ gap: 10 }}>
        <Section title="Revenue" rows={grouped.income} currency={currency} onAccount={onAccount} total={incomeTotal} />
        <Section title="Expenses" rows={grouped.expenses} currency={currency} onAccount={onAccount} total={expenseTotal} />
        <Card><Text style={styles.name}>Net Profit</Text><Text style={styles.amount}>{formatDisplay(net, currency)}</Text></Card>
      </View>
    );
  }
  if (report === "balance-sheet") {
    const grouped = groupBalanceSheet(items);
    return (
      <View style={{ gap: 10 }}>
        <Section title="Assets" rows={grouped.assets} currency={currency} onAccount={onAccount} />
        <Section title="Liabilities" rows={grouped.liabilities} currency={currency} onAccount={onAccount} />
        <Section title="Equity" rows={grouped.equity} currency={currency} onAccount={onAccount} />
        <Section title="Current Earnings" rows={grouped.earnings} currency={currency} onAccount={onAccount} />
        {currentEarnings ? <Card><Text style={styles.name}>Current Earnings</Text><Text style={styles.amount}>{formatDisplay(currentEarnings, currency)}</Text></Card> : null}
      </View>
    );
  }
  if (report === "trial-balance") {
    return <View style={{ gap: 8 }}>{items.map((row, i) => <Card key={i}><Text style={styles.name}>{row.code} {row.name}</Text><Text>Debit {row.debit || row.debits || "0"}</Text><Text>Credit {row.credit || row.credits || "0"}</Text></Card>)}</View>;
  }
  if (report === "cash-movement") {
    return <View style={{ gap: 8 }}>{items.map((row, i) => <Card key={i}><Text>{row.date}</Text><Text style={styles.name}>{row.name}</Text><Text>{formatDisplay(row.movement || "0", row.currency || currency)}</Text></Card>)}</View>;
  }
  if (report === "sales-analysis") {
    return <View style={{ gap: 8 }}>{items.map((row, i) => <Card key={i}><Text style={styles.name}>{row.account_name || row.name}</Text><Text>{row.sales_channel_name || "Channel"} · {row.customer_segment || "Segment"} · {row.route_name || "Route"}</Text><Text>{formatDisplay(row.amount || "0", currency)}</Text></Card>)}</View>;
  }
  return <View style={{ gap: 8 }}>{items.map((row, i) => (
    <Pressable key={i} onPress={() => row.account_id && onAccount(row.account_id)}>
      <Card><Text>{row.date}</Text><Text style={styles.name}>{row.account_code} {row.account_name}</Text><Text>{row.description}</Text><Text>Debit {row.debit || "0"} · Credit {row.credit || "0"}</Text></Card>
    </Pressable>
  ))}</View>;
}

function Section({ title, rows, currency, onAccount, total }: { title: string; rows: Row[]; currency: string; onAccount: (id: string) => void; total?: string }) {
  return (
    <View style={{ gap: 8 }}>
      <Text style={styles.section}>{title}</Text>
      {rows.length === 0 ? <Text style={styles.muted}>None</Text> : null}
      {total ? <Text style={styles.amount}>{formatDisplay(total, currency)}</Text> : null}
      {rows.map((row, i) => (
        <Pressable key={i} onPress={() => (row.id || row.account_id) && onAccount(String(row.id || row.account_id))}>
          <Card><Text style={styles.name}>{row.code} {row.name}</Text><Text style={styles.amount}>{formatDisplay(row.amount || row.balance || "0", currency)}</Text></Card>
        </Pressable>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12, paddingBottom: 40 },
  row: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  chip: { minHeight: 44, justifyContent: "center", paddingHorizontal: 12, borderRadius: 999, backgroundColor: "#fff", borderWidth: 1, borderColor: colors.line },
  chipOn: { backgroundColor: colors.ink },
  on: { color: "#fff", fontWeight: "700" },
  off: { color: colors.ink },
  section: { fontWeight: "800", fontSize: 18, color: colors.ink },
  muted: { color: colors.muted },
  name: { fontWeight: "700", color: colors.ink, flexShrink: 1 },
  amount: { fontVariant: ["tabular-nums"], fontWeight: "800", fontSize: 18 },
});
