# Tester guide

How to run SPARC Horizon's working prototype, what to try, what to expect, and how to report what
you find. It takes about fifteen minutes and needs nothing beyond Docker, or Go to run from source.

Every output quoted below was measured against an image built from this repository (#117, #120),
not written from the contract. If what you see differs, that is worth reporting. To look at **your
own** OSCAL instead of the synthetic federation, go to [section 7](#7-bring-your-own-oscal).

## What you are testing

A **mock** of Horizon's API v0 over a **synthetic** federation: four organizations, seven
authorization boundaries and twenty systems, published as 32 OSCAL 1.2.3 documents in
[`fixtures/`](../fixtures/README.md). The same process serves the HUD demos.

| Real today | Not real yet |
|---|---|
| The read endpoints and their response shapes ([`api/openapi.yaml`](../api/openapi.yaml)) | Projection is state at a date, from the documents, with no ledger: the image defaults to the fixtures' **2026-10-15**, and `?horizon=YYYY-MM-DD` moves it |
| The refusal rule: a node you cannot see is **404**, the same as one that does not exist | Login: a header picks the persona. It is **not** authentication |
| Each persona's tree, computed from the OSCAL documents' role bindings | The write endpoints (attestations, decisions, what-if) and blast radius answer **501** |
| The OSCAL export, with a `sha256` per document that verifies what you fetched | Ranking: next action is a placeholder, not the ranking model |
| The image: no shell, uid 1001, a read-only served tree | A hosted environment: you run it yourself |

The design behind each row is in [`docs/04-api.md`](04-api.md) and
[`docs/07-security.md`](07-security.md).

## 1. Run it

**With Docker:**

```bash
git clone https://github.com/risk-sentinel/sparc-horizon.git && cd sparc-horizon
docker build -t sparc-horizon:dev .
docker run --rm -p 8080:8080 sparc-horizon:dev
```

**Or from source** (Go 1.27): `go run ./cmd/mockserver -static demo`.

Either way it logs the three personas and listens on port 8080:

```text
mockserver: serving /srv/horizon/demo at /
mockserver: 3 personas, serving /srv/horizon/fixtures on http://0.0.0.0:8080
  X-Horizon-Persona: ao-ods                             Office of Digital Services Authorizing Official
  X-Horizon-Persona: so-ods-portal                      Public Portal System Owner
  X-Horizon-Persona: iso-bgm-grants                     Grants Platform Information System Security Officer
```

(From source, the paths are `demo` and `fixtures` and it listens on `127.0.0.1:8080`.)

Check it: `curl -s http://localhost:8080/healthz` returns `{"status":"ok"}`, and
<http://localhost:8080/> opens the demos. A published, signed image will be available after the
first release; [`docs/08-build-deploy.md`](08-build-deploy.md#release) shows how to verify its
signature with `cosign verify` before you run it.

The commands below use `jq` to keep the output short. Set these once:

```bash
B=http://localhost:8080
AO='X-Horizon-Persona: ao-ods'
SO='X-Horizon-Persona: so-ods-portal'
ISO='X-Horizon-Persona: iso-bgm-grants'
ORG=cdf7d9f6-cf10-5944-a0ef-0351bf1ec41a      # Office of Digital Services
PORTAL=f7de308a-5555-5e35-be6a-029ad885320e   # Public Portal boundary
GRANTS=a8a226ff-c477-5c42-abdb-a46e356db5d8   # Grants Platform boundary
```

## 2. Each persona sees a different tree

```bash
for p in "$AO" "$SO" "$ISO"; do
  curl -s -H "$p" $B/v1/tree | jq -c '[.. | objects | select(has("nodeType")) | .name]'
done
```

```text
["Office of Digital Services","Identity Services","Agency Identity Provider","Directory Service","Multi-Factor Service","Public Portal","Portal Web Tier","Portal API","Portal Edge Cache"]
["Public Portal","Portal Web Tier","Portal API","Portal Edge Cache"]
["Grants Platform","Grants Application","Grants Database","Grants ETL"]
```

The authorizing official holds a role on the organization and sees everything beneath it. The
system owner holds one boundary. The ISO is in a different organization and shares nothing with
either. Each tree is derived from `responsible-parties` in the SSPs, not stored.

**Try to see what is not yours.** The system owner asking for the organization, the ISO asking
for the Portal, and anyone asking for a node that never existed all get the same answer:

```bash
curl -s -w ' %{http_code}\n' -H "$SO"  $B/v1/nodes/$ORG/heat
curl -s -w ' %{http_code}\n' -H "$ISO" $B/v1/nodes/$PORTAL/heat
curl -s -w ' %{http_code}\n' -H "$SO"  $B/v1/nodes/00000000-0000-5000-8000-000000000000/heat
```

```text
{"detail":"No such node, or you hold no role on it. These are deliberately indistinguishable.","error":"not_found"}
 404
```

(three times: the body ends in a newline, so the status prints on the next line). A 403 would confirm that the node exists; the 404 does not. An unknown persona is a
401: `curl -s -H 'X-Horizon-Persona: nobody' $B/v1/tree`.

## 3. From the heatmap to the evidence

Follow one problem down, as the Public Portal's system owner.

**Heat.** Rows are the Portal's systems and columns are 800-53 families. Families with nothing
wrong collapse, so only SI is drawn:

```bash
curl -s -H "$SO" $B/v1/nodes/$PORTAL/heat \
  | jq -c '{columns, collapsed, rows: [.rows[] | {name, cells: [.cells[] | {col, state}]}]}'
```

```text
{"columns":["SI"],"collapsed":["AC","AU","CP","IA","SC"],"rows":[{"name":"Portal Web Tier","cells":[]},{"name":"Portal API","cells":[{"col":"SI","state":"degraded"}]},{"name":"Portal Edge Cache","cells":[{"col":"SI","state":"nominal"}]}]}
```

A row with no cell in a column has nothing assessed in that family. That is a gap, not a pass.

**Cell.** What is down in SI, and why:

```bash
curl -s -H "$SO" "$B/v1/nodes/$PORTAL/cell?col=SI" | jq -c '.controls[] | {controlId, state, reason, chain}'
```

```text
{"controlId":"si-4","state":"degraded","reason":"Evidence expires 2026-09-19, before the 2026-10-15 horizon.","chain":"adcd691d-fb75-5e59-b369-bf017cf06e72"}
```

**Next action** agrees: `curl -s -H "$SO" $B/v1/nodes/$PORTAL/next-action | jq -r .title` prints
`Resolve si-4 on Public Portal`.

**Chain.** The evidence behind that control, by implemented-requirement UUID:

```bash
curl -s -H "$SO" $B/v1/controls/adcd691d-fb75-5e59-b369-bf017cf06e72/chain | jq .
```

```json
{
  "requirement": "adcd691d-fb75-5e59-b369-bf017cf06e72",
  "controlId": "si-4",
  "component": "e0c4e151-6e8c-5bcf-8be5-eb674b498b2e",
  "resource": "f5d86538-4d5a-5dc4-99ae-720f07b49b98",
  "observation": "f0e4a6e0-ecbe-5023-b3dd-2a2508f9c682",
  "expires": "2026-09-19",
  "blocksAto": false
}
```

Each UUID is an object in the boundary's OSCAL documents: the component is in the SSP, and the
observation is in the assessment results. A chain whose component you cannot see is a 404, the
same as a node.

**The same walk as the AO** starts one tier up. The organization's heat has rows per boundary,
`curl -s -H "$AO" $B/v1/nodes/$ORG/heat | jq -c .columns` prints `["IA","SI","SC"]`, and its next
action is a blocker: `Resolve ia-5 on Identity Services`, *"An open risk on this control blocks the
authorization for Identity Services."* Its chain carries a `finding`, a `risk` and a `poamItem`,
and `blocksAto` is `true`.

## 4. Export the OSCAL and verify it

The export lists a boundary's SSP, assessment results and POA&M, each with a `sha256`. Name the
boundary by slug or by its UUID:

```bash
curl -s -H "$SO" $B/v1/export/oscal/ods-portal | jq -c '.documents[] | {model, href}'
curl -s -H "$SO" $B/v1/export/oscal/$PORTAL | jq -r .boundary     # ods-portal
```

```text
{"model":"system-security-plan","href":"/oscal/ssp-ods-portal.json"}
{"model":"assessment-results","href":"/oscal/ar-ods-portal.json"}
{"model":"plan-of-action-and-milestones","href":"/oscal/poam-ods-portal.json"}
```

Fetch a document and check it against the hash the export gave you:

```bash
curl -s -H "$SO" $B/oscal/ar-ods-portal.json | shasum -a 256
curl -s -H "$SO" $B/v1/export/oscal/ods-portal | jq -r '.documents[1].sha256'
```

Both print `c2eb09697f526d43b05ddcfc12fd157f64ad534b06c579debf6c614d640755f9`. Another
organization's boundary, or its documents, is the same 404 as everywhere else:
`curl -s -w ' %{http_code}\n' -H "$SO" $B/v1/export/oscal/bgm-grants`.

The documents are standard OSCAL 1.2.3, so any OSCAL tool can read them. To validate one against
NIST's schema, use the `ajv-cli` command in [`docs/compliance/README.md`](compliance/README.md).

## 5. What answers 501, and why

```bash
curl -s -w ' %{http_code}\n' -X POST -H "$SO" $B/v1/attestations
```

```text
{"detail":"The mock serves the read endpoints. POST /v1/attestations is in the frozen contract and is implemented by the service, not here.","error":"not_implemented"}
 501
```

The same goes for `POST /v1/attestations/{id}/sign`, `POST /v1/decisions`, `POST /v1/whatif` and
`GET /v1/nodes/{id}/blast-radius`. A mock that accepted a write and then forgot it would be worse
than one that refuses. The [roadmap](roadmap.md) says which phase builds each of them.

## 6. The demos

<http://localhost:8080/> lists them. [`hud.html`](../demo/hud.html) is the heatmap with the
look-ahead slider, drill-in and what-if. [`planner.html`](../demo/planner.html) is the phase
schedule. The demos carry their own synthetic data and **do not call the API yet**: P3 rebuilds the
HUD against it. Judge them as interaction design, and the API as the data they will show.

## 7. Bring your own OSCAL

Horizon reads OSCAL only; translating scanner output (HDF, SARIF and the like) into OSCAL happens
before it, in SPARC or your own pipeline. Put your **SSP, assessment plan, assessment results and
POA&M** (JSON, OSCAL 1.1.2 to 1.2.3) in one folder, from SPARC's export or any other producer, and
point the server at it:

```bash
docker run --rm -p 8080:8080 -v "$PWD/my-oscal:/data:ro" sparc-horizon:dev \
  -addr 0.0.0.0:8080 -dir /data -static /srv/horizon/demo
# or from source: go run ./cmd/mockserver -dir my-oscal -static demo
```

Everything is computed from the documents when the server starts: the tree from the SSPs' parties
and `responsible-parties`, and each control's state from the standard relationships (finding →
observation → component, finding → risk → POA&M item). File names don't matter. The horizon
defaults to today; add `-horizon YYYY-MM-DD` to change it.

**Who you can be.** Without a `personas.json`, there is one persona per party holding a role the
contract knows: `authorizing-official`, `system-owner` or `information-system-security-officer`.
The startup log lists them. Send the persona id or the party's UUID in `X-Horizon-Persona`. If your
documents name roles differently, map them: `-role-alias owner=system-owner` (repeatable). The
persona header is still not authentication; your documents decide who sees what, which is fine for a
local mock and is not how real ingest will work.

**Read the startup log.** Every link that did not resolve is listed, with the file and what was
done about it. A document that could not be placed in any SSP is not shown, and the log says so.

NIST's published example set is in the repository, so you can try this now:

```bash
docker run --rm -p 8080:8080 -v "$PWD/internal/oscal/testdata/nist:/data:ro" sparc-horizon:dev \
  -addr 0.0.0.0:8080 -dir /data -static /srv/horizon/demo -role-alias owner=system-owner -horizon 2023-06-15
```

```text
mockserver: 1 SSP, 1 assessment plan, 1 assessment results, 1 POA&M, 1 component definition; 1 boundaries, 1 assessed control-component pairs, 1 other JSON files ignored, 9 problems
  problem: ifa_assessment-results.json: its assessment plan ifa_assessment-plan.json imports "../../ssp/xml/ifa_ssp.xml", which is not a loaded SSP
  problem: ifa_assessment-results.json: placed in ifa_ssp-example.json by the components its observations name
  ...
  problem: 8101e04d-8305-4e73-bb95-6b59f645b143: tree: implicit-federation: organization 3a675986-b4ff-4030-b178-e953c2e55d64 names no federation; placed under "Local federation"
  ...
mockserver: 1 personas, default horizon 2023-06-15, serving /data on http://0.0.0.0:8080
  X-Horizon-Persona: so-owen-stilskin                   Owen Stilskin
```

NIST's files link to each other by XML path, so each placement falls back and says how. Then:

```bash
SO='X-Horizon-Persona: so-owen-stilskin'; SSP=5e139edd-86aa-4b65-8431-1192bd276658
curl -s -H "$SO" $B/v1/nodes/$SSP/heat | jq -c '{columns, rows: [.rows[] | {name, cells: [.cells[] | {col, state}]}]}'
curl -s -H "$SO" $B/v1/nodes/$SSP/next-action | jq -r .title
curl -s -H "$SO" $B/v1/controls/d5f9b263-965d-440b-99e7-77f5df670a11/chain | jq -c .
```

```text
{"columns":["AC"],"rows":[{"name":"IFA GoodRead System","cells":[{"col":"AC","state":"degraded"}]}]}
Resolve ac-6.1 on IFA GoodRead
{"requirement":"d5f9b263-965d-440b-99e7-77f5df670a11","controlId":"ac-6.1","component":"551b9706-d6a4-4d25-8207-f2ccec548b89","observation":"8807eb6e-0c05-43bc-8438-799739615e34","expires":"2023-07-01","finding":"45d8a6c2-1368-4bad-9ba0-7141f0a32889","risk":"0cfa750e-3553-47ba-a7ba-cf84a884d261","blocksAto":false,"poamItem":"48c8368d-43ff-4736-9b28-64b1b1284c03"}
```

A control your assessment results never mention is a **gap, not a pass**: it has no cell. If your
documents join and nothing shows, check the log for `could not be placed`, and that your findings
carry `target.target-id` (for example `ac-2_obj`) and `related-observations` with `subjects`.

## Reporting what you find

Open an issue at <https://github.com/risk-sentinel/sparc-horizon/issues> and include:

- **What you ran:** the commit (`git rev-parse --short HEAD`), or the image digest
  (`docker image inspect sparc-horizon:dev --format '{{.Id}}'`, or the `@sha256:` you pulled);
- **As whom:** the persona header you sent;
- **The request and the response:** the exact `curl` command, the status code, and the body;
- **What you expected**, and where that expectation came from: this guide, the contract, or the
  demos.

Useful findings include a response that disagrees with another (a cell whose controls the chain
does not back up, say), any way to see a node outside your tree, a response that does not match
`api/openapi.yaml`, and anything in this guide that did not happen as written. For a security
issue, use GitHub's private vulnerability reporting on the repository rather than a public issue.
