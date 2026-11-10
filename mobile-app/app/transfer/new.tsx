import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Stack, useRouter } from "expo-router";
import { useMemo, useState } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, Text } from "react-native";
import { api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { useOffline } from "../../src/components/NetworkBanner";
import { Button, SelectField, TextField } from "../../src/components/ui";
import { todayInTimeZone } from "../../src/lib/dates";
import { createKeyStore } from "../../src/lib/idempotency";
import { friendlyError } from "../../src/api/errors";
import { transferErrors, transferPayload, TransferDraft } from "../../src/lib/transfer";
import { colors, space } from "../../src/theme";

type Fin = { PublicID: string; Name: string; Currency: string; Active: boolean };

export default function TransferScreen() {
  const { entity } = useEntity();
  const router = useRouter();
  const offline = useOffline();
  const client = useQueryClient();
  const keys = useMemo(() => createKeyStore(), []);
  const [draft, setDraft] = useState<TransferDraft>({
    date: entity ? todayInTimeZone(entity.Timezone) : "",
    description: "",
    fromId: "",
    toId: "",
    fromAmount: "",
    toAmount: "",
    fromCurrency: "",
    toCurrency: "",
  });
  const [error, setError] = useState("");
  const fins = useQuery({
    queryKey: ["fins", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Fin[] }>(`/entities/${entity!.PublicID}/financial-accounts`),
  });
  const accounts = (fins.data?.items ?? []).filter((item) => item.Active);
  const problems = transferErrors(draft);
  const save = useMutation({
    mutationFn: () => {
      const body = transferPayload(draft);
      return api().request<{ id: string }>(`/entities/${entity!.PublicID}/transfers`, {
        method: "POST",
        idempotencyKey: keys.keyFor(JSON.stringify(body)),
        body: JSON.stringify(body),
      });
    },
    onSuccess: async (result) => {
      await client.invalidateQueries({ queryKey: ["balances"] });
      await client.invalidateQueries({ queryKey: ["transactions"] });
      router.replace(`/transaction/${result.id}`);
    },
    onError: (err) => setError(friendlyError(err)),
  });
  function pick(which: "from" | "to", id: string) {
    const account = accounts.find((item) => item.PublicID === id);
    setDraft((current) => ({
      ...current,
      [which === "from" ? "fromId" : "toId"]: id,
      [which === "from" ? "fromCurrency" : "toCurrency"]: account?.Currency || "",
    }));
  }
  return (
    <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1, backgroundColor: colors.paper }}>
      <Stack.Screen options={{ title: "Transfer" }} />
      <ScrollView contentContainerStyle={styles.body}>
        <Text style={styles.note}>Transfers post immediately. Cross-currency amounts use Finance exchange rates. This screen does not calculate a rate.</Text>
        <TextField label="Date" value={draft.date} onChangeText={(date) => setDraft({ ...draft, date })} />
        <TextField label="Description" value={draft.description} onChangeText={(description) => setDraft({ ...draft, description })} />
        <SelectField label="From account" value={draft.fromId} onChange={(id) => pick("from", id)} options={accounts.map((item) => ({ id: item.PublicID, label: `${item.Name} (${item.Currency})` }))} />
        <SelectField label="To account" value={draft.toId} onChange={(id) => pick("to", id)} options={accounts.map((item) => ({ id: item.PublicID, label: `${item.Name} (${item.Currency})` }))} />
        <TextField label={`Amount out ${draft.fromCurrency}`} value={draft.fromAmount} onChangeText={(fromAmount) => setDraft({ ...draft, fromAmount, toAmount: draft.fromCurrency && draft.fromCurrency === draft.toCurrency ? fromAmount : draft.toAmount })} keyboardType="decimal-pad" />
        <TextField label={`Amount in ${draft.toCurrency}`} value={draft.toAmount} onChangeText={(toAmount) => setDraft({ ...draft, toAmount })} keyboardType="decimal-pad" />
        {problems[0] ? <Text style={styles.note}>{problems[0]}</Text> : null}
        {error ? <Text style={styles.error}>{error}</Text> : null}
        <Button label="Transfer" disabled={offline || save.isPending || problems.length > 0} onPress={() => save.mutate()} />
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12 },
  note: { color: colors.muted },
  error: { color: colors.negative, fontWeight: "700" },
});
