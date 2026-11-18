import assert from "node:assert/strict";
import { describe, test } from "node:test";

import {
  STATES,
  addDays,
  byWorst,
  cellLabel,
  countdown,
  daysBetween,
  displayControl,
  drillable,
  familyName,
  findNode,
  formatRoute,
  isIsoDate,
  isState,
  names,
  parseRoute,
  pathTo,
  shortId,
  today,
} from "./model.ts";
import type { TreeNode } from "./types.ts";

const tree: TreeNode = {
  id: "00000000-0000-0000-0000-000000000001",
  name: "Org",
  nodeType: "organization",
  roles: ["authorizing-official"],
  children: [
    {
      id: "00000000-0000-0000-0000-000000000002",
      name: "Boundary",
      nodeType: "boundary",
      roles: [],
      children: [{ id: "00000000-0000-0000-0000-000000000003", name: "System", nodeType: "system", roles: [] }],
    },
    { id: "00000000-0000-0000-0000-000000000004", name: "Empty boundary", nodeType: "boundary", roles: [] },
  ],
};

describe("states", () => {
  test("every state has a distinct glyph, so colour is never the only signal", () => {
    const glyphs = Object.values(STATES).map((s) => s.glyph);
    assert.equal(new Set(glyphs).size, 4);
    for (const g of glyphs) assert.notEqual(g.trim(), "");
  });

  test("the contract's four states and nothing else", () => {
    for (const s of ["nominal", "watch", "degraded", "blocks"]) assert.ok(isState(s), s);
    for (const s of ["", "ok", "BLOCKS", "toString", "__proto__"]) assert.ok(!isState(s), s);
  });

  test("worst first", () => {
    const sorted = (["nominal", "blocks", "watch", "degraded"] as const).map((state) => ({ state })).sort(byWorst);
    assert.deepEqual(
      sorted.map((x) => x.state),
      ["blocks", "degraded", "watch", "nominal"],
    );
  });

  test("a cell's accessible name carries everything its colour and glyph do", () => {
    assert.equal(
      cellLabel("Portal", { col: "IA", state: "blocks", blockers: 2, worsening: true, inherited: true }),
      "Portal, IA Identification and Authentication: Blocks ATO, 2 blocking the authorization, worsening, inherited",
    );
    assert.equal(
      cellLabel("Portal", { col: "XX", state: "nominal", blockers: 0, worsening: false, inherited: false }),
      "Portal, XX XX: Nominal",
    );
  });
});

describe("controls and families", () => {
  test("families by code, any case; unknown codes pass through", () => {
    assert.equal(familyName("si"), "System and Information Integrity");
    assert.equal(familyName("EC2"), "EC2");
  });

  test("NIST display form", () => {
    assert.equal(displayControl("ac-2"), "AC-2");
    assert.equal(displayControl("ac-2.1"), "AC-2(1)");
    assert.equal(displayControl("AC-6.10"), "AC-6(10)");
    assert.equal(displayControl("EC2.2"), "EC2.2");
    assert.equal(displayControl("ac-2(1)"), "ac-2(1)");
  });
});

describe("dates", () => {
  test("only real calendar dates", () => {
    assert.ok(isIsoDate("2026-10-15"));
    assert.ok(isIsoDate("2028-02-29"));
    for (const s of ["2026-02-30", "2027-02-29", "2026-1-5", "20261015", "", "2026-10-15T00:00:00Z", "bad"]) {
      assert.ok(!isIsoDate(s), s);
    }
  });

  test("day arithmetic is calendar arithmetic in UTC", () => {
    assert.equal(addDays("2026-10-15", 30), "2026-11-14");
    assert.equal(addDays("2026-12-31", 1), "2027-01-01");
    assert.equal(addDays("2026-03-01", -1), "2026-02-28");
    assert.equal(daysBetween("2026-10-15", "2027-10-15"), 365);
    assert.equal(daysBetween("2026-10-15", "2026-10-01"), -14);
  });

  test("countdowns, past and future", () => {
    assert.equal(countdown("2026-10-15", "2026-10-15"), "today");
    assert.equal(countdown("2026-10-16", "2026-10-15"), "tomorrow");
    assert.equal(countdown("2026-10-14", "2026-10-15"), "1 day ago");
    assert.equal(countdown("2026-11-28", "2026-10-15"), "in 44 days");
    assert.equal(countdown("2026-09-19", "2026-10-15"), "26 days ago");
  });

  test("today is the UTC date", () => {
    assert.equal(today(new Date("2026-11-18T23:30:00-05:00")), "2026-11-19");
  });
});

describe("tree", () => {
  test("the path to a node, root first", () => {
    assert.deepEqual(
      pathTo(tree, "00000000-0000-0000-0000-000000000003")?.map((n) => n.name),
      ["Org", "Boundary", "System"],
    );
    assert.equal(pathTo(tree, "00000000-0000-0000-0000-00000000dead"), undefined);
    assert.equal(findNode(tree, "00000000-0000-0000-0000-000000000002")?.name, "Boundary");
  });

  test("systems and empty nodes are not drillable", () => {
    assert.ok(drillable(tree));
    assert.ok(!drillable({ nodeType: "system", children: [tree] }));
    assert.ok(!drillable({ nodeType: "boundary" }));
    assert.ok(!drillable({ nodeType: "boundary", children: [] }));
  });

  test("names by UUID, every tier", () => {
    const n = names(tree);
    assert.equal(n.size, 4);
    assert.equal(n.get("00000000-0000-0000-0000-000000000003"), "System");
  });

  test("short ids", () => {
    assert.equal(shortId("443ef4ad-dcde-538e-9782-0746b3ae253b"), "443ef4ad…");
    assert.equal(shortId("abc"), "abc");
  });
});

describe("route", () => {
  const full = {
    persona: "so-ods-portal",
    node: "f7de308a-5555-5e35-be6a-029ad885320e",
    horizon: "2026-10-15",
    row: "e0c4e151-6e8c-5bcf-8be5-eb674b498b2e",
    col: "SI",
    chain: "beed167a-28d5-5cfe-aaef-14917b3d938c",
  };

  test("round-trips", () => {
    assert.deepEqual(parseRoute(formatRoute(full)), full);
    assert.equal(formatRoute({}), "#");
    assert.deepEqual(parseRoute(""), {});
  });

  test("columns normalise to upper case", () => {
    assert.equal(parseRoute("#c=si").col, "SI");
  });

  test("malformed values are dropped, never sent on", () => {
    const r = parseRoute(
      "#p=" + encodeURIComponent("<img src=x>") + "&n=../../etc&h=2026-02-30&r=x&c=" + encodeURIComponent("SI;drop") + "&x=" + "a".repeat(37),
    );
    assert.deepEqual(r, {});
  });

  test("undefined fields are omitted", () => {
    assert.equal(formatRoute({ persona: "ao", node: undefined, col: "" }), "#p=ao");
  });
});
