/**
 * Fail-closed production API host check for MyanKafe Finance.
 * Documented public API: https://finance-api.myankafe.com
 * (docs/DEPLOYMENT_ACCEPTANCE_REPORT.md)
 */
const PRODUCTION_API_ORIGIN = "https://finance-api.myankafe.com";
const PRODUCTION_API_BASE_URL = `${PRODUCTION_API_ORIGIN}/api/v1`;

function assertProductionApiBaseUrl(raw) {
  const value = String(raw || "").trim().replace(/\/$/, "");
  let url;
  try {
    url = new URL(value);
  } catch {
    throw new Error("production API must be https://finance-api.myankafe.com/api/v1");
  }
  const host = url.hostname.toLowerCase();
  const blocked =
    host === "localhost" ||
    host === "127.0.0.1" ||
    host === "0.0.0.0" ||
    host === "10.0.2.2" ||
    host.endsWith(".local") ||
    host.includes("staging") ||
    host.includes("example");
  if (url.protocol !== "https:" || blocked || host !== "finance-api.myankafe.com") {
    throw new Error("production API must be https://finance-api.myankafe.com/api/v1");
  }
  const path = url.pathname.replace(/\/$/, "") || "";
  if (path !== "/api/v1") {
    throw new Error("production API must be https://finance-api.myankafe.com/api/v1");
  }
  return `${url.origin}${path}`;
}

module.exports = {
  PRODUCTION_API_ORIGIN,
  PRODUCTION_API_BASE_URL,
  assertProductionApiBaseUrl,
};
