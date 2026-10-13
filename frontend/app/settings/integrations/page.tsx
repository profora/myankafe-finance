"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useEntity } from "@/components/EntityContext";
import { canConfigureAccounting, canManageInterEntitySetup } from "@/lib/permissions";
import TableStateRows from "@/components/TableStateRows";

type Connection = { public_id: string; name: string; active: boolean };
type Mapping = { source_key: string; mapping_kind: string; target_public_id: string; target_code: string; target_name: string };
type EventRow = {
  public_id: string;
  external_event_id: string;
  event_type: string;
  status: string;
  accounting_date: string;
  transaction_public_id: string;
  error_code: string;
  error_message: string;
};
type Readiness = {
  connection_active: boolean;
  secret_configured: boolean;
  entity_name: string;
  accounting_start_date: string | null;
  locked_through: string | null;
  mapping_counts: Record<string, number>;
  missing_mapping_keys: string[];
  ready: boolean;
};
type Account = { PublicID: string; Code: string; Name: string; Postable: boolean; Active: boolean };
type FinancialAccount = { PublicID: string; Code: string; Name: string; Currency: string; Active: boolean };
type SalesChannel = { id: string; code: string; name: string; active: boolean };

export default function IntegrationsPage() {
  const { entity } = useEntity();
  const mayConfigure = canConfigureAccounting(entity?.Role);
  const mayActivate = canManageInterEntitySetup(entity?.Role);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [mappings, setMappings] = useState<Mapping[]>([]);
  const [suggested, setSuggested] = useState<string[]>([]);
  const [events, setEvents] = useState<EventRow[]>([]);
  const [readiness, setReadiness] = useState<Readiness | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [financialAccounts, setFinancialAccounts] = useState<FinancialAccount[]>([]);
  const [channels, setChannels] = useState<SalesChannel[]>([]);
  const [sourceKey, setSourceKey] = useState("AR");
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  async function load() {
    if (!entity) return;
    setLoading(true);
    setError("");
    try {
      const [conn, maps, ev, ready, acc, fa, ch] = await Promise.all([
        api<{ items: Connection[]; secret_configured: boolean }>(`/entities/${entity.PublicID}/integrations/connections`),
        api<{ items: Mapping[]; suggested_keys: string[] }>(`/entities/${entity.PublicID}/integrations/mappings`),
        api<{ items: EventRow[] }>(`/entities/${entity.PublicID}/integrations/events`),
        api<Readiness>(`/entities/${entity.PublicID}/integrations/readiness`),
        api<{ items: Account[] }>(`/entities/${entity.PublicID}/accounts`),
        api<{ items: FinancialAccount[] }>(`/entities/${entity.PublicID}/financial-accounts`),
        api<{ items: SalesChannel[] }>(`/entities/${entity.PublicID}/sales-channels`),
      ]);
      setConnections(conn.items ?? []);
      setMappings(maps.items ?? []);
      setSuggested(maps.suggested_keys ?? []);
      setEvents(ev.items ?? []);
      setReadiness(ready);
      setAccounts(acc.items ?? []);
      setFinancialAccounts(fa.items ?? []);
      setChannels(ch.items ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    setConnections([]);
    setMappings([]);
    setEvents([]);
    setReadiness(null);
    if (entity) void load();
  }, [entity?.PublicID]);

  const connection = connections[0];
  const kind = sourceKey.startsWith("PAYMENT_METHOD:")
    ? "FINANCIAL_ACCOUNT"
    : sourceKey.startsWith("SALES_CHANNEL:")
      ? "SALES_CHANNEL"
      : "ACCOUNT";

  async function createConnection() {
    if (!entity) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await api(`/entities/${entity.PublicID}/integrations/connections`, { method: "POST", body: JSON.stringify({ name: "MyanKafe Platform" }) });
      setMessage("Connection created inactive. The shared secret stays in the server environment and is never shown here.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function setActive(active: boolean) {
    if (!entity) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await api(`/entities/${entity.PublicID}/integrations/connections/MYANKAFE_PLATFORM`, {
        method: "PUT",
        body: JSON.stringify({ name: connection?.name || "MyanKafe Platform", active }),
      });
      setMessage(active ? "Connection activated." : "Connection deactivated.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function saveMapping() {
    if (!entity || !sourceKey || !target) return;
    setBusy(true);
    setError("");
    setMessage("");
    const body =
      kind === "FINANCIAL_ACCOUNT"
        ? { financial_account_public_id: target }
        : kind === "SALES_CHANNEL"
          ? { sales_channel_public_id: target }
          : { account_public_id: target };
    try {
      await api(`/entities/${entity.PublicID}/integrations/mappings/${encodeURIComponent(sourceKey)}`, {
        method: "PUT",
        body: JSON.stringify(body),
      });
      setMessage("Mapping saved.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  if (!mayConfigure) return <div className="alert">Integration configuration requires OWNER, ADMIN, or ACCOUNTANT.</div>;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>MyanKafe Platform</h1>
          <p>Finance owns the account mapping. Platform sends business events. The shared secret is never displayed.</p>
        </div>
      </div>
      {error && <div className="alert error" role="alert">{error}</div>}
      {message && <div className="alert success" role="status">{message}</div>}

      <div className="card" style={{ marginBottom: 16 }}>
        <h3>Connection</h3>
        <p>Status: {connection ? (connection.active ? "Active" : "Inactive") : "Not created"}</p>
        <p>Secret configured: {readiness?.secret_configured ? "Yes" : "No"}</p>
        <p>Accounting start: {readiness?.accounting_start_date || "Not set"}</p>
        <p>Locked through: {readiness?.locked_through || "Open"}</p>
        <p>Readiness: {readiness?.ready ? "Ready" : "Not ready"}</p>
        {!connection && mayActivate && <button type="button" disabled={busy} onClick={createConnection}>Create inactive connection</button>}
        {connection && mayActivate && (
          <button type="button" className="secondary" disabled={busy} onClick={() => setActive(!connection.active)}>
            {connection.active ? "Deactivate" : "Activate"}
          </button>
        )}
      </div>

      <div className="card form" style={{ marginBottom: 16 }}>
        <h3>Save a mapping</h3>
        <p>Suggested keys are not created automatically. Save only the mappings you intend to use.</p>
        <div className="form-grid">
          <div className="field">
            <label>Source key</label>
            <input list="integration-keys" value={sourceKey} onChange={(e) => { setSourceKey(e.target.value.toUpperCase()); setTarget(""); }} />
            <datalist id="integration-keys">
              {suggested.map((key) => <option key={key} value={key} />)}
            </datalist>
          </div>
          <div className="field">
            <label>Target</label>
            <select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Select</option>
              {kind === "ACCOUNT" && accounts.filter((a) => a.Active && a.Postable).map((a) => <option key={a.PublicID} value={a.PublicID}>{a.Code} {a.Name}</option>)}
              {kind === "FINANCIAL_ACCOUNT" && financialAccounts.filter((a) => a.Active && a.Currency === "MMK").map((a) => <option key={a.PublicID} value={a.PublicID}>{a.Code} {a.Name}</option>)}
              {kind === "SALES_CHANNEL" && channels.filter((a) => a.active).map((a) => <option key={a.id} value={a.id}>{a.code} {a.name}</option>)}
            </select>
          </div>
        </div>
        <button type="button" disabled={busy || !target} onClick={saveMapping}>Save mapping</button>
      </div>

      <div className="table-wrap" style={{ marginBottom: 16 }} aria-busy={loading}>
        <table>
          <thead><tr><th>Source</th><th>Kind</th><th>Target</th></tr></thead>
          <tbody>
            <TableStateRows loading={loading} columns={3} empty={!loading && mappings.length === 0} emptyText="No mappings saved" />
            {mappings.map((row) => <tr key={row.source_key}><td>{row.source_key}</td><td>{row.mapping_kind}</td><td>{row.target_code} {row.target_name}</td></tr>)}
          </tbody>
        </table>
      </div>

      {readiness && readiness.missing_mapping_keys.length > 0 && (
        <div className="alert">Missing suggested mappings: {readiness.missing_mapping_keys.join(", ")}</div>
      )}

      <div className="table-wrap" aria-busy={loading}>
        <table>
          <thead><tr><th>Event</th><th>Type</th><th>Status</th><th>Date</th><th>Transaction</th><th>Error</th></tr></thead>
          <tbody>
            <TableStateRows loading={loading} columns={6} empty={!loading && events.length === 0} emptyText="No integration events" />
            {events.map((row) => (
              <tr key={row.public_id}>
                <td>{row.external_event_id}</td>
                <td>{row.event_type}</td>
                <td>{row.status}</td>
                <td>{row.accounting_date}</td>
                <td>{row.transaction_public_id}</td>
                <td>{row.error_message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
