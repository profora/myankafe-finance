import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Image } from "expo-image";
import { Stack, useLocalSearchParams, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { Alert, ScrollView, StyleSheet, Text, View } from "react-native";
import { api, appExtra } from "../../src/api/runtime";
import { getSessionToken } from "../../src/auth/session";
import { useEntity } from "../../src/auth/EntityContext";
import { useOffline } from "../../src/components/NetworkBanner";
import { Button, Card, ErrorState, LoadingState, StatusBadge, TextField } from "../../src/components/ui";
import { todayInTimeZone } from "../../src/lib/dates";
import { formatDisplay } from "../../src/lib/money";
import { canCorrectPostedAccounting, canOperateLedger } from "../../src/lib/roles";
import { createKeyStore } from "../../src/lib/idempotency";
import { friendlyError } from "../../src/api/errors";
import { colors, space } from "../../src/theme";

type Detail = {
  id: string;
  type: string;
  status: string;
  date: string;
  description: string;
  currency: string;
  total: string;
  contact?: { name?: string | null };
  financial_account?: { name?: string | null };
  sales_channel?: { name?: string | null };
  original_transaction_id?: string | null;
  reversal_transaction_id?: string | null;
  void_reason?: string | null;
  journals?: { id: string; status: string; lines: { account: { code: string; name: string }; debit: string; credit: string }[] }[];
};
type Attachment = { id: string; original_filename: string; mime_type: string; size_bytes: number };

export default function TransactionDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { entity } = useEntity();
  const router = useRouter();
  const offline = useOffline();
  const client = useQueryClient();
  const [token, setToken] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [cancelReason, setCancelReason] = useState("");
  const [actionError, setActionError] = useState("");
  const [reversalDate, setReversalDate] = useState(entity ? todayInTimeZone(entity.Timezone) : "");
  const keys = useRef(createKeyStore());
  const detail = useQuery({
    queryKey: ["transaction", entity?.PublicID, id],
    enabled: Boolean(entity && id),
    queryFn: async () => {
      setToken(await getSessionToken());
      return api().request<Detail>(`/entities/${entity!.PublicID}/transactions/${id}`);
    },
  });
  const files = useQuery({
    queryKey: ["attachments", entity?.PublicID, id],
    enabled: Boolean(entity && id),
    queryFn: () => api().request<{ items: Attachment[] }>(`/entities/${entity!.PublicID}/transactions/${id}/attachments`),
  });
  const act = useMutation({
    mutationFn: async (action: "post" | "cancel" | "reverse") => {
      const body =
        action === "reverse"
          ? { reversal_date: reversalDate, reason: reason.trim() }
          : action === "cancel"
            ? { reason: cancelReason.trim() }
            : {};
      const key = keys.current.keyFor(`${action}:${JSON.stringify(body)}`);
      return api().request(`/entities/${entity!.PublicID}/transactions/${id}/${action}`, {
        method: "POST",
        idempotencyKey: key,
        body: JSON.stringify(body),
      });
    },
    onSuccess: async () => {
      setActionError("");
      keys.current.reset();
      await client.invalidateQueries({ queryKey: ["transaction", entity?.PublicID, id] });
      await client.invalidateQueries({ queryKey: ["transactions"] });
      await client.invalidateQueries({ queryKey: ["balances"] });
      await client.invalidateQueries({ queryKey: ["dashboard"] });
      await client.invalidateQueries({ queryKey: ["report"] });
    },
    onError: (err) => setActionError(friendlyError(err)),
  });
  const tx = detail.data;
  const mayOperate = canOperateLedger(entity?.Role) && !offline;
  const mayReverse = canCorrectPostedAccounting(entity?.Role) && !offline;
  return (
    <ScrollView contentContainerStyle={styles.body}>
      <Stack.Screen options={{ title: tx?.status === "POSTED" ? "Posted entry" : "Transaction" }} />
      {detail.isLoading ? <LoadingState /> : detail.isError || !tx ? <ErrorState message="Transaction not found." /> : (
        <>
          <Card>
            <StatusBadge status={tx.status} />
            <Text style={styles.id}>Public ID {tx.id}</Text>
            <Text style={styles.desc}>{tx.description}</Text>
            <Text>{tx.date} · {tx.type}</Text>
            <Text>Account {tx.financial_account?.name || "—"}</Text>
            <Text>Contact {tx.contact?.name || "—"}</Text>
            {tx.sales_channel?.name ? <Text>Sales channel {tx.sales_channel.name}</Text> : null}
            <Text style={styles.total}>{formatDisplay(tx.total, tx.currency)}</Text>
            <Text>Source {tx.type}</Text>
            {tx.original_transaction_id ? <Text>Reverses {tx.original_transaction_id}</Text> : null}
            {tx.reversal_transaction_id ? <Text>Reversed by {tx.reversal_transaction_id}</Text> : null}
            {tx.void_reason ? <Text>Void reason {tx.void_reason}</Text> : null}
          </Card>
          {(tx.journals ?? []).map((journal) => (
            <Card key={journal.id}>
              <Text style={styles.desc}>Journal {journal.status}</Text>
              {journal.lines.map((line, index) => (
                <View key={index}>
                  <Text>{line.account.code} {line.account.name}</Text>
                  <Text>Debit {line.debit} · Credit {line.credit}</Text>
                </View>
              ))}
            </Card>
          ))}
          <Text style={styles.desc}>Attachments</Text>
          {(files.data?.items ?? []).length === 0 ? <Text style={styles.meta}>None</Text> : null}
          {(files.data?.items ?? []).map((file) => (
            <Card key={file.id}>
              <Text>{file.original_filename}</Text>
              <Text style={styles.meta}>{file.mime_type} · {file.size_bytes} bytes</Text>
              {file.mime_type.startsWith("image/") && token ? (
                <Image style={styles.image} source={{ uri: `${appExtra().apiBaseUrl}/entities/${entity!.PublicID}/transactions/${id}/attachments/${file.id}/content`, headers: { Authorization: `Bearer ${token}` } }} />
              ) : null}
            </Card>
          ))}
          {actionError ? <Text style={styles.error}>{actionError}</Text> : null}
          {tx.status === "DRAFT" && mayOperate ? (
            <>
              <Button label="Edit" tone="secondary" onPress={() => router.push({ pathname: "/transaction/new", params: { edit: tx.id } })} />
              <Button label="Post" disabled={act.isPending} onPress={() => Alert.alert("Post transaction", "Post this draft to the ledger?", [{ text: "Cancel", style: "cancel" }, { text: "Post", onPress: () => { if (!act.isPending) act.mutate("post"); } }])} />
              <TextField label="Cancellation reason" value={cancelReason} onChangeText={setCancelReason} placeholder="At least 3 characters" />
              <Button label="Cancel draft" tone="danger" disabled={act.isPending || cancelReason.trim().length < 3} onPress={() => Alert.alert("Cancel draft", "Void this draft? This stays in the audit log.", [{ text: "Keep", style: "cancel" }, { text: "Cancel draft", style: "destructive", onPress: () => { if (!act.isPending) act.mutate("cancel"); } }])} />
            </>
          ) : null}
          {tx.status === "POSTED" && mayReverse ? (
            <Card>
              <Text style={styles.desc}>Reverse transaction</Text>
              <Text style={styles.meta}>This posts a reversing entry. It cannot be undone from the phone.</Text>
              <TextField label="Reversal date" value={reversalDate} onChangeText={setReversalDate} />
              <TextField label="Reason" value={reason} onChangeText={setReason} />
              <Button label="Reverse Transaction" tone="danger" disabled={!reason.trim() || act.isPending} onPress={() => Alert.alert("Confirm reversal", "This posts a reversing entry. It is not a swipe action and cannot be undone from the phone.", [{ text: "Back", style: "cancel" }, { text: "Reverse", style: "destructive", onPress: () => { if (!act.isPending) act.mutate("reverse"); } }])} />
            </Card>
          ) : null}
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12, backgroundColor: colors.paper },
  id: { color: colors.muted },
  desc: { fontSize: 18, fontWeight: "800", color: colors.ink },
  total: { fontSize: 24, fontWeight: "800", fontVariant: ["tabular-nums"] },
  meta: { color: colors.muted },
  error: { color: colors.negative, fontWeight: "700" },
  image: { width: "100%", height: 220, borderRadius: 12 },
});
