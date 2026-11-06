import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Asset } from "expo-asset";
import * as DocumentPicker from "expo-document-picker";
import * as ImagePicker from "expo-image-picker";
import { Stack, useLocalSearchParams, useRouter } from "expo-router";
import { ReactNode, useEffect, useMemo, useState } from "react";
import { Alert, KeyboardAvoidingView, Platform, ScrollView, StyleSheet, Text, View } from "react-native";
import { appExtra, api } from "../../src/api/runtime";
import { useEntity } from "../../src/auth/EntityContext";
import { useOffline } from "../../src/components/NetworkBanner";
import { Button, SelectField, TextField } from "../../src/components/ui";
import { attachmentError } from "../../src/lib/attachments";
import { todayInTimeZone } from "../../src/lib/dates";
import { EntryDraft, entryErrors, entryTotal, removeSplit, saveEntry, SplitDraft } from "../../src/lib/entry";
import { createIdempotencyKey } from "../../src/lib/idempotency";
import { friendlyError } from "../../src/api/errors";
import { colors, space } from "../../src/theme";

type Account = { PublicID: string; Code: string; Name: string; Type: string; Postable: boolean; Active: boolean };
type Fin = { PublicID: string; Name: string; Currency: string; Active: boolean };
type Contact = { id: string; display_name: string };
type Channel = { id: string; name: string; active: boolean };
type LocalFile = { name: string; uri: string; mime?: string | null; size?: number | null };

export default function NewEntryScreen() {
  const { entity } = useEntity();
  const params = useLocalSearchParams<{ type?: string; edit?: string }>();
  const router = useRouter();
  const offline = useOffline();
  const client = useQueryClient();
  const [type, setType] = useState<"INCOME" | "EXPENSE">(params.type === "INCOME" ? "INCOME" : "EXPENSE");
  const [date, setDate] = useState(entity ? todayInTimeZone(entity.Timezone) : "");
  const [description, setDescription] = useState("");
  const [financialAccountId, setFinancialAccountId] = useState("");
  const [contactQuery, setContactQuery] = useState("");
  const [contactId, setContactId] = useState("");
  const [salesChannelId, setSalesChannelId] = useState("");
  const [splits, setSplits] = useState<SplitDraft[]>([{ accountId: "", amount: "", description: "" }]);
  const [files, setFiles] = useState<LocalFile[]>([]);
  const [error, setError] = useState("");
  const [key, setKey] = useState(() => createIdempotencyKey());

  const accounts = useQuery({
    queryKey: ["accounts", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Account[] }>(`/entities/${entity!.PublicID}/accounts`),
  });
  const fins = useQuery({
    queryKey: ["fins", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Fin[] }>(`/entities/${entity!.PublicID}/financial-accounts`),
  });
  const contacts = useQuery({
    queryKey: ["contacts", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Contact[] }>(`/entities/${entity!.PublicID}/contacts`),
  });
  const channels = useQuery({
    queryKey: ["channels", entity?.PublicID],
    enabled: Boolean(entity),
    queryFn: () => api().request<{ items: Channel[] }>(`/entities/${entity!.PublicID}/sales-channels`),
  });

  const activeChannels = (channels.data?.items ?? []).filter((c) => c.active);
  const visibleContacts = (contacts.data?.items ?? []).filter((item) =>
    item.display_name.toLowerCase().includes(contactQuery.trim().toLowerCase()),
  );
  const ledger = (accounts.data?.items ?? []).filter((a) => a.Active && a.Postable && a.Type === (type === "INCOME" ? "INCOME" : "EXPENSE"));
  const fin = (fins.data?.items ?? []).find((item) => item.PublicID === financialAccountId);
  const draft: EntryDraft = useMemo(
    () => ({
      type,
      date,
      description,
      financialAccountId,
      currency: fin?.Currency || entity?.FunctionalCurrency || "",
      contactId,
      salesChannelId,
      salesChannelsAvailable: type === "INCOME" && activeChannels.length > 0,
      splits,
    }),
    [type, date, description, financialAccountId, fin?.Currency, entity?.FunctionalCurrency, contactId, salesChannelId, activeChannels.length, splits],
  );
  const total = entryTotal(draft);

  useEffect(() => {
    if (!entity || !params.edit) return;
    void api().request<{ date: string; description: string; type: string; currency: string; financial_account?: { id?: string }; contact?: { id?: string }; sales_channel?: { id?: string }; splits: { account_id: string; amount: string; description: string }[] }>(`/entities/${entity.PublicID}/transactions/${params.edit}`).then((tx) => {
      setType(tx.type === "INCOME" ? "INCOME" : "EXPENSE");
      setDate(tx.date);
      setDescription(tx.description);
      setFinancialAccountId(tx.financial_account?.id || "");
      setContactId(tx.contact?.id || "");
      setSalesChannelId(tx.sales_channel?.id || "");
      setSplits(tx.splits.map((split) => ({ accountId: split.account_id, amount: split.amount, description: split.description })));
    });
  }, [entity, params.edit]);

  function addFile(file: LocalFile) {
    const problem = attachmentError(file);
    if (problem) {
      setError(problem);
      return;
    }
    if (files.length >= 10) {
      setError("A transaction can attach up to 10 files at a time.");
      return;
    }
    setError("");
    setFiles((current) => [...current, file]);
  }

  const save = useMutation({
    mutationFn: (mode: "draft" | "post") =>
      saveEntry({
        mode,
        draft,
        idempotencyKey: key,
        attachments: files,
        create: (body, idempotencyKey) =>
          params.edit
            ? api().request<{ PublicID: string }>(`/entities/${entity!.PublicID}/transactions/${params.edit}`, {
                method: "PUT",
                body: JSON.stringify(body),
                idempotencyKey,
              })
            : api().request<{ PublicID: string }>(`/entities/${entity!.PublicID}/transactions`, {
                method: "POST",
                body: JSON.stringify(body),
                idempotencyKey,
              }),
        upload: async (id, attachments) => {
          const body = new FormData();
          attachments.forEach((file, index) => {
            const local = files[index];
            body.append("files", { uri: local.uri, name: file.name, type: local.mime || "application/octet-stream" } as unknown as Blob);
          });
          await api().request(`/entities/${entity!.PublicID}/transactions/${id}/attachments`, { method: "POST", body });
        },
        post: (id, idempotencyKey) =>
          api().request(`/entities/${entity!.PublicID}/transactions/${id}/post`, { method: "POST", body: "{}", idempotencyKey }),
      }),
    onSuccess: async (result) => {
      await client.invalidateQueries({ queryKey: ["transactions"] });
      await client.invalidateQueries({ queryKey: ["balances"] });
      await client.invalidateQueries({ queryKey: ["dashboard"] });
      if (!result.ok && result.draftKept) {
        Alert.alert("Draft kept", result.message, [{ text: "Open draft", onPress: () => router.replace(`/transaction/${result.id}`) }]);
        return;
      }
      if (!result.ok) {
        if (/idempotency/i.test(result.message)) setKey(createIdempotencyKey());
        setError(result.message);
        return;
      }
      router.replace(`/transaction/${result.id}`);
    },
    onError: (err) => {
      const message = friendlyError(err);
      if (message.toLowerCase().includes("idempotency")) setKey(createIdempotencyKey());
      setError(message);
    },
  });

  const problems = entryErrors(draft);

  return (
    <KeyboardAvoidingView behavior={Platform.OS === "ios" ? "padding" : undefined} style={{ flex: 1, backgroundColor: colors.paper }}>
      <Stack.Screen options={{ title: params.edit ? "Edit draft" : "New Entry" }} />
      <ScrollView contentContainerStyle={styles.body} keyboardShouldPersistTaps="handled">
        <SelectField label="Type" value={type} onChange={(id) => setType(id as "INCOME" | "EXPENSE")} options={[{ id: "INCOME", label: "Income" }, { id: "EXPENSE", label: "Expense" }]} />
        <TextField label="Date" value={date} onChangeText={setDate} placeholder="YYYY-MM-DD" />
        <TextField label="Description" value={description} onChangeText={setDescription} />
        <SelectField
          label={type === "INCOME" ? "Received into" : "Paid from"}
          value={financialAccountId}
          onChange={setFinancialAccountId}
          options={(fins.data?.items ?? []).filter((item) => item.Active).map((item) => ({ id: item.PublicID, label: `${item.Name} (${item.Currency})` }))}
        />
        <TextField label={type === "INCOME" ? "Payer" : "Payee"} value={contactQuery} onChangeText={setContactQuery} placeholder="Search contacts" />
        <SelectField
          label="Contact"
          value={contactId}
          onChange={setContactId}
          options={[{ id: "", label: "None" }, ...visibleContacts.map((item) => ({ id: item.id, label: item.display_name }))]}
        />
        {contactQuery && visibleContacts.length === 0 ? <Text style={styles.note}>No matching contact. Create it in Finance web, then search again.</Text> : null}
        {type === "INCOME" && activeChannels.length > 0 ? (
          <SelectField label="Sales channel" value={salesChannelId} onChange={setSalesChannelId} options={activeChannels.map((item) => ({ id: item.id, label: item.name }))} />
        ) : null}
        {splits.map((split, index) => (
          <ViewBlock key={index}>
            <SelectField label={`Split ${index + 1} account`} value={split.accountId} onChange={(id) => setSplits((rows) => rows.map((row, i) => i === index ? { ...row, accountId: id } : row))} options={ledger.map((item) => ({ id: item.PublicID, label: `${item.Code} ${item.Name}` }))} />
            <TextField label="Amount" value={split.amount} onChangeText={(amount) => setSplits((rows) => rows.map((row, i) => i === index ? { ...row, amount } : row))} keyboardType="decimal-pad" />
            <TextField label="Line description" value={split.description} onChangeText={(description) => setSplits((rows) => rows.map((row, i) => i === index ? { ...row, description } : row))} />
            {splits.length > 1 ? <Button label={`Remove split ${index + 1}`} tone="secondary" onPress={() => setSplits((rows) => removeSplit(rows, index))} /> : null}
          </ViewBlock>
        ))}
        <Button label="Add split" tone="secondary" onPress={() => setSplits((rows) => [...rows, { accountId: "", amount: "", description: "" }])} />
        <Text style={styles.total}>Total {total ?? "—"} {draft.currency}</Text>
        {files.map((file) => <Text key={file.uri}>{file.name}</Text>)}
        <Button label="Camera" tone="secondary" onPress={async () => {
          try {
            const permission = await ImagePicker.requestCameraPermissionsAsync();
            if (!permission.granted) {
              setError("Camera permission is required to attach a receipt.");
              return;
            }
            const shot = await ImagePicker.launchCameraAsync({ mediaTypes: ["images"], quality: 0.8 });
            if (!shot.canceled) addFile({ name: "camera.jpg", uri: shot.assets[0].uri, mime: shot.assets[0].mimeType || "image/jpeg", size: shot.assets[0].fileSize });
          } catch {
            setError("Could not open the camera.");
          }
        }} />
        <Button label="Photo library" tone="secondary" onPress={async () => {
          try {
            const permission = await ImagePicker.requestMediaLibraryPermissionsAsync();
            if (!permission.granted) {
              setError("Photo library permission is required to attach a receipt.");
              return;
            }
            const shot = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ["images"], quality: 0.8 });
            if (!shot.canceled) addFile({ name: shot.assets[0].fileName || "photo.jpg", uri: shot.assets[0].uri, mime: shot.assets[0].mimeType, size: shot.assets[0].fileSize });
          } catch {
            setError("Could not open the photo library.");
          }
        }} />
        <Button label="File" tone="secondary" onPress={async () => {
          try {
            const picked = await DocumentPicker.getDocumentAsync({ copyToCacheDirectory: false, multiple: false });
            if (!picked.canceled) addFile({ name: picked.assets[0].name, uri: picked.assets[0].uri, mime: picked.assets[0].mimeType, size: picked.assets[0].size });
          } catch {
            setError("Could not open the file picker.");
          }
        }} />
        {appExtra().appEnv !== "production" ? (
          <Button
            label="Attach sample receipt"
            tone="secondary"
            onPress={async () => {
              try {
                const asset = Asset.fromModule(require("../../assets/e2e-receipt.png"));
                await asset.downloadAsync();
                if (!asset.localUri) {
                  setError("Sample receipt is not available.");
                  return;
                }
                addFile({ name: "e2e-receipt.png", uri: asset.localUri, mime: "image/png", size: asset.localUri ? undefined : null });
              } catch {
                setError("Could not attach the sample receipt.");
              }
            }}
          />
        ) : null}
        {error ? <Text style={styles.error}>{error}</Text> : null}
        {problems.length ? <Text style={styles.note}>{problems[0]}</Text> : null}
        <Button label="Save Draft" disabled={offline || save.isPending || problems.length > 0} onPress={() => save.mutate("draft")} />
        <Button label="Save & Post" disabled={offline || save.isPending || problems.length > 0} onPress={() => Alert.alert("Post transaction", "Post this entry to the ledger?", [{ text: "Cancel", style: "cancel" }, { text: "Post", onPress: () => save.mutate("post") }])} />
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

function ViewBlock({ children }: { children: ReactNode }) {
  return <View style={styles.block}>{children}</View>;
}

const styles = StyleSheet.create({
  body: { padding: space.md, gap: 12, paddingBottom: 48 },
  block: { gap: 8 },
  total: { fontSize: 22, fontWeight: "800", color: colors.ink, fontVariant: ["tabular-nums"] },
  note: { color: colors.muted },
  error: { color: colors.negative, fontWeight: "700" },
});
