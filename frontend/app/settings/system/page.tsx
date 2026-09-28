"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

type SystemStatus = {
  app_env: string;
  database: string;
  attachment_storage_configured: boolean;
  metrics_available: boolean;
  metrics_protected: boolean;
};

type ProbeResult = {
  ok: boolean;
  duration_ms: number;
};

export default function SystemSettings() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [probe, setProbe] = useState<ProbeResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    setError("");
    try {
      setStatus(await api<SystemStatus>("/system/status"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function runProbe() {
    setBusy(true);
    setError("");
    setProbe(null);
    try {
      const result = await api<ProbeResult>("/system/storage-probe", {
        method: "POST",
        body: "{}",
      });
      setProbe(result);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>System</h1>
          <p>OWNER-only runtime diagnostics. No secrets are displayed here.</p>
        </div>
        <button type="button" className="secondary" disabled={loading} onClick={load}>{loading?"Refreshing…":"Refresh"}</button>
      </div>

      {error && <div className="alert error" role="alert">{error}</div>}
      {probe?.ok && (
        <div className="alert success" role="status" aria-live="polite">
          R2 write/read/delete probe passed in {probe.duration_ms.toFixed(1)} ms.
        </div>
      )}

      <div className="grid cards" aria-busy={loading}>
        <div className="card">
          <div className="muted">Environment</div>
          <div className="metric system-metric">{loading?<span className="skeleton skeleton-line" aria-hidden="true"/>:(status?.app_env ?? "Unavailable")}</div>
        </div>
        <div className="card">
          <div className="muted">PostgreSQL</div>
          <div className="metric system-metric">{loading?<span className="skeleton skeleton-line" aria-hidden="true"/>:(status?.database === "ok" ? "Ready" : "Unavailable")}</div>
        </div>
        <div className="card">
          <div className="muted">R2 attachment storage</div>
          <div className="metric system-metric">
            {loading?<span className="skeleton skeleton-line" aria-hidden="true"/>:(status ? (status.attachment_storage_configured ? "Configured" : "Not configured") : "Unavailable")}
          </div>
        </div>
        <div className="card">
          <div className="muted">Metrics endpoint</div>
          <div className="metric system-metric">
            {loading?<span className="skeleton skeleton-line" aria-hidden="true"/>:(status ? (status.metrics_available ? (status.metrics_protected ? "Protected" : "Available") : "Hidden") : "Unavailable")}
          </div>
        </div>
      </div>

      <div className="card system-probe-card">
        <div>
          <h3>R2 acceptance probe</h3>
          <p className="muted">
            Writes a tiny temporary object, reads it back, verifies the bytes, then deletes it. The operation is audited.
          </p>
        </div>
        <button type="button" disabled={loading || busy || !status?.attachment_storage_configured} onClick={runProbe}>
          {busy ? "Testing R2..." : "Run R2 probe"}
        </button>
      </div>
    </>
  );
}
