import assert from "node:assert/strict";
import test from "node:test";
import {
  accountMutationControls,
  addChildDraft,
  afterAccountSave,
  blankDraft,
  buildAccountTree,
  changeDraftType,
  editDraft,
  parentOptions,
  type TreeAccount,
} from "./accountTree.ts";

function account(partial: Partial<TreeAccount> & Pick<TreeAccount, "PublicID" | "Code" | "Name" | "Type">): TreeAccount {
  return {
    Subtype: "",
    Postable: true,
    Active: true,
    ParentPublicID: null,
    ...partial,
  };
}

const revenue = account({ PublicID: "rev", Code: "4000", Name: "Revenue", Type: "INCOME", Postable: false });
const coffee = account({ PublicID: "coffee", Code: "4100", Name: "Coffee Sales", Type: "INCOME", Postable: false, ParentPublicID: "rev" });
const twoPlus = account({ PublicID: "two", Code: "4110", Name: "2Plus1 Sales", Type: "INCOME", ParentPublicID: "coffee" });
const powder = account({ PublicID: "powder", Code: "4120", Name: "Pure Arabica Powder Sales", Type: "INCOME", ParentPublicID: "coffee" });
const other = account({ PublicID: "other", Code: "4900", Name: "Other Income", Type: "INCOME", ParentPublicID: "rev" });
const expense = account({ PublicID: "exp", Code: "5000", Name: "Cost of Sales", Type: "EXPENSE", Postable: false });
const posting = account({ PublicID: "post", Code: "5110", Name: "Coffee Cost", Type: "EXPENSE", ParentPublicID: "exp" });
const inactive = account({ PublicID: "old", Code: "4300", Name: "Retired", Type: "INCOME", Postable: false, Active: false });
const accounts = [powder, revenue, posting, twoPlus, other, expense, coffee, inactive];

test("new account opens a create modal", () => {
  const draft = blankDraft();
  assert.equal(draft.mode, "create");
  assert.equal(draft.parentPublicID, "");
  assert.equal(draft.postable, true);
});

test("edit opens a modal populated from the account", () => {
  const draft = editDraft(twoPlus);
  assert.equal(draft.mode, "edit");
  assert.equal(draft.code, "4110");
  assert.equal(draft.name, "2Plus1 Sales");
  assert.equal(draft.type, "INCOME");
  assert.equal(draft.parentPublicID, "coffee");
  assert.equal(draft.postable, true);
  assert.equal(draft.active, true);
});

test("add child preselects the header type and parent", () => {
  const draft = addChildDraft(coffee);
  assert.equal(draft.mode, "create");
  assert.equal(draft.type, "INCOME");
  assert.equal(draft.parentPublicID, "coffee");
});

test("parent options are active headers of the same type", () => {
  const options = parentOptions(accounts, "INCOME", "two");
  assert.deepEqual(options.map(item => item.Code), ["4000", "4100"]);
});

test("edit parent options exclude the account and its descendants", () => {
  const options = parentOptions(accounts, "INCOME", "coffee");
  assert.deepEqual(options.map(item => item.Code), ["4000"]);
});

test("changing type clears a parent that is no longer valid", () => {
  const next = changeDraftType({ ...blankDraft(), type: "INCOME", parentPublicID: "coffee" }, "EXPENSE", accounts);
  assert.equal(next.type, "EXPENSE");
  assert.equal(next.parentPublicID, "");
});

test("successful save closes the modal and an API error stays open", () => {
  const draft = editDraft(twoPlus);
  assert.equal(afterAccountSave(draft, ""), null);
  const failed = afterAccountSave(draft, "parent account must be a header account");
  assert.equal(failed?.error, "parent account must be a header account");
  assert.equal(failed?.publicID, "two");
});

test("a read-only role has no account mutation controls", () => {
  assert.deepEqual(accountMutationControls("VIEWER"), { create: false, edit: false, addChild: false });
  assert.deepEqual(accountMutationControls("BOOKKEEPER"), { create: false, edit: false, addChild: false });
  assert.equal(accountMutationControls("ACCOUNTANT").create, true);
});

test("the chart is ordered by code within each parent and survives a cycle", () => {
  const tree = buildAccountTree(accounts);
  assert.deepEqual(tree.map(row => `${" ".repeat(row.depth)}${row.account.Code}`), [
    "4000",
    " 4100",
    "  4110",
    "  4120",
    " 4900",
    "4300",
    "5000",
    " 5110",
  ]);

  const loopA = account({ PublicID: "a", Code: "1000", Name: "A", Type: "ASSET", Postable: false, ParentPublicID: "b" });
  const loopB = account({ PublicID: "b", Code: "1100", Name: "B", Type: "ASSET", Postable: false, ParentPublicID: "a" });
  const orphan = account({ PublicID: "c", Code: "1200", Name: "C", Type: "ASSET", ParentPublicID: "missing" });
  const safe = buildAccountTree([loopA, loopB, orphan]);
  assert.equal(safe.length, 3);
  assert.deepEqual(safe.map(row => row.account.Code).sort(), ["1000", "1100", "1200"]);
});
