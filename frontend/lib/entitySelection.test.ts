import assert from "node:assert/strict";
import test from "node:test";
import { chooseAccessibleEntity, entityResourceListPath, withEntity } from "./entitySelection.ts";

const myan = "01MYAN";
const royal = "01ROYAL";
const personal = "01PERSONAL";
const accessible = [myan, personal, royal];

test("query entity wins over storage and the first entity", () => {
  assert.equal(chooseAccessibleEntity(accessible, [royal, myan]), royal);
});

test("stored entity is used when the query is missing or inaccessible", () => {
  assert.equal(chooseAccessibleEntity(accessible, ["", personal]), personal);
  assert.equal(chooseAccessibleEntity(accessible, ["01MISSING", personal]), personal);
});

test("inaccessible storage falls back to the first accessible entity", () => {
  assert.equal(chooseAccessibleEntity(accessible, ["01MISSING", "01REVOKED"]), myan);
  assert.equal(chooseAccessibleEntity([], ["01MISSING"]), "");
});

test("resource links keep the entity query", () => {
  assert.equal(withEntity(`/transactions/${royal}`, myan), `/transactions/${royal}?entity=${myan}`);
  assert.equal(withEntity("/accounts/abc/ledger?from=2026-01-01", royal), `/accounts/abc/ledger?from=2026-01-01&entity=${royal}`);
});

test("changing entity on a detail page returns to that entity's list", () => {
  assert.equal(entityResourceListPath("/transactions/01TX"), "/transactions");
  assert.equal(entityResourceListPath("/transactions/new"), null);
  assert.equal(entityResourceListPath("/accounts/01ACCT/ledger"), "/accounts");
  assert.equal(entityResourceListPath("/transactions"), null);
  assert.equal(entityResourceListPath("/"), null);
});
