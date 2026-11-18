import assert from "node:assert/strict";
import { describe, test } from "node:test";

import { Api, ApiError, PERSONA_HEADER, personas, query } from "./api.ts";

interface Call {
  url: string;
  headers: Record<string, string>;
}

function fake(status: number, body: unknown, calls: Call[] = []): (url: string, init?: RequestInit) => Promise<Response> {
  return (url, init) => {
    calls.push({ url, headers: { ...(init?.headers as Record<string, string> | undefined) } });
    const text = typeof body === "string" ? body : JSON.stringify(body);
    return Promise.resolve(new Response(text, { status, headers: { "Content-Type": "application/json" } }));
  };
}

describe("query", () => {
  test("skips undefined and empty values, encodes the rest", () => {
    assert.equal(query({}), "");
    assert.equal(query({ horizon: undefined }), "");
    assert.equal(query({ col: "SI", horizon: "" }), "?col=SI");
    assert.equal(query({ col: "a&b=c" }), "?col=a%26b%3Dc");
  });
});

describe("Api", () => {
  test("names the caller on every request", async () => {
    const calls: Call[] = [];
    const api = new Api("so-ods-portal", fake(200, {}, calls));
    await api.tree();
    await api.heat("n1", "2026-10-15");
    await api.cell("n1", "SI");
    await api.nextAction("n1");
    await api.chain("c1");
    await api.export("b1");
    assert.deepEqual(
      calls.map((c) => c.url),
      [
        "/v1/tree",
        "/v1/nodes/n1/heat?horizon=2026-10-15",
        "/v1/nodes/n1/cell?col=SI",
        "/v1/nodes/n1/next-action",
        "/v1/controls/c1/chain",
        "/v1/export/oscal/b1",
      ],
    );
    for (const c of calls) assert.equal(c.headers[PERSONA_HEADER], "so-ods-portal");
  });

  test("no header when there is no persona to name", async () => {
    const calls: Call[] = [];
    await new Api("", fake(200, {}, calls)).tree();
    assert.equal(calls[0]?.headers[PERSONA_HEADER], undefined);
  });

  test("path segments are encoded, so an id cannot reach another path", async () => {
    const calls: Call[] = [];
    await new Api("p", fake(200, {}, calls)).heat("../tree?x=1");
    assert.equal(calls[0]?.url, "/v1/nodes/..%2Ftree%3Fx%3D1/heat");
  });

  test("errors carry the contract's code, and 404 reads as not visible", async () => {
    const api = new Api("p", fake(404, { error: "not_found", detail: "No such node" }));
    await assert.rejects(api.tree(), (e: unknown) => {
      assert.ok(e instanceof ApiError);
      assert.equal(e.status, 404);
      assert.equal(e.code, "not_found");
      assert.equal(e.message, "No such node");
      assert.ok(e.notVisible);
      return true;
    });
  });

  test("a non-JSON error body still yields the status", async () => {
    await assert.rejects(new Api("p", fake(502, "<html>bad gateway</html>")).tree(), (e: unknown) => {
      assert.ok(e instanceof ApiError);
      assert.equal(e.code, "http_502");
      assert.ok(!e.notVisible);
      return true;
    });
  });
});

describe("personas", () => {
  test("the mock's list", async () => {
    const list = await personas(fake(200, [{ id: "ao", name: "AO" }]));
    assert.equal(list.length, 1);
  });

  test("a server without the mock's list has none to offer", async () => {
    assert.deepEqual(await personas(fake(404, { error: "not_found" })), []);
  });

  test("other failures are failures", async () => {
    await assert.rejects(personas(fake(500, { error: "boom" })), ApiError);
  });
});
