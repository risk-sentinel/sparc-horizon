#!/usr/bin/env python3
"""Write the per-run evidence provenance record.

The problem this exists for (#204, #208, dev-sec-ops-baseline#21): a scan that
found nothing and a bridge that silently broke produce the same artifact — an HDF
with zero controls, which renders as `compliance: 0`. Neither obvious fix works
alone. Emitting the empty profile reports a false negative; skipping the emit
makes clean and broken indistinguishable.

So the *findings* stay a scanner result, faithfully converted, and the claim
"this ran, here is what it observed" becomes a separate, explicit record written
on EVERY run — including clean ones, including runs where every conversion
failed.

The discriminator is deliberate:

    status=ok            + findings=0     -> clean scan
    status=convert-failed| input-missing  -> broken bridge, findings=null

`findings` is null rather than 0 whenever the count is unknown, because a 0 that
means "we could not tell" is exactly the confusion this file exists to prevent.
That is the same failure mode as aws-config#7, where a guard counted controls and
passed a file with 78 controls and not one NIST tag: presence was measured,
the thing that mattered was not.

Usage:
    provenance.py --lane sca-source --out provenance.json \\
        --scanner name=grype,version=v0.114.0,status=ok,input=g.json,hdf=hdf/g.json,findings=3 \\
        --scanner name=trivy,version=v0.72.0,status=convert-failed,input=t.json,error='no such file' \\
        --gate severity=CRITICAL,gating=0,allowlisted=1 \\
        [--emit attempted=false,reason='no emit role supplied'] \\
        [--require-hdf]

`--require-hdf` exits 1 when a scanner declared `status=ok` without a readable
HDF. Used by the smoke tests to make a broken bridge loud; NOT used by the
reusables, where evidence must never gate.
"""
from __future__ import annotations

import argparse
import datetime as _dt
import hashlib
import json
import os
import re
import sys
from pathlib import Path

SCHEMA = "risk-sentinel/evidence-provenance/v1"

VALID_STATUS = {"ok", "convert-failed", "input-missing", "skipped"}

# Keys this script interprets itself. Everything else in a --scanner spec is
# passed through, so a lane can record its own facts without editing this file.
RESERVED = {"name", "version", "status", "input", "hdf", "findings", "error"}


def die(msg: str) -> None:
    print(f"::error::provenance: {msg}", file=sys.stderr)
    sys.exit(1)


def _confined(path: str) -> str:
    """Resolve a caller-supplied path, refusing anything outside the working dir.

    Paths here come from workflow inputs. Confining the canonical target under
    CWD sanitizes before any filesystem access (SonarPython S2083), matching
    tools/dispositions/expand_dispositions.py."""
    base = os.path.realpath(os.getcwd())
    target = os.path.realpath(path)
    if base != target and os.path.commonpath([base, target]) != base:
        die(f"refusing to touch {path!r} — resolves outside {base}")
    return target


def _sha256(path: str) -> str | None:
    try:
        h = hashlib.sha256()
        with open(_confined(path), "rb") as fh:
            for chunk in iter(lambda: fh.read(1 << 20), b""):
                h.update(chunk)
        return h.hexdigest()
    except OSError:
        return None


# Split on a comma only when the next token looks like the start of a new
# `key=`. Values legitimately contain commas — `severity=CRITICAL,HIGH` and
# `platforms=Dockerfile,CICD` are both real — and a naive split turned the first
# into the fragment 'HIGH' and killed the run.
_FRAGMENT = re.compile(r",(?=[A-Za-z_][A-Za-z0-9_.-]*=)")


def _kv(spec: str, what: str) -> dict:
    """Parse `k=v,k=v`. Values may contain '=' and ','."""
    out: dict[str, str] = {}
    for part in _FRAGMENT.split(spec):
        part = part.strip()
        if not part:
            continue
        if "=" not in part:
            die(f"malformed --{what} fragment {part!r}; expected key=value")
        k, v = part.split("=", 1)
        out[k.strip()] = v.strip()
    return out


def _int_or_none(v):
    if v is None or v == "":
        return None
    try:
        return int(v)
    except ValueError:
        return None


def build_scanner(spec: str) -> dict:
    d = _kv(spec, "scanner")
    name = d.get("name")
    if not name:
        die(f"--scanner {spec!r} has no name=")

    status = d.get("status", "skipped")
    if status not in VALID_STATUS:
        die(f"--scanner {name}: status={status!r} not in {sorted(VALID_STATUS)}")

    hdf_path = d.get("hdf") or None
    inp = d.get("input") or None

    # findings is null unless the conversion actually succeeded. A count from a
    # failed conversion is not a count of anything.
    findings = _int_or_none(d.get("findings")) if status == "ok" else None

    rec = {
        "name": name,
        "version": d.get("version") or None,
        "status": status,
        "input": inp,
        "input_sha256": _sha256(inp) if inp else None,
        "hdf": hdf_path,
        "hdf_sha256": _sha256(hdf_path) if hdf_path else None,
        "findings": findings,
    }
    if d.get("error"):
        rec["error"] = d["error"]

    # Anything else the caller passed is carried through verbatim. Lanes have
    # scanner-specific facts worth recording that this script should not need to
    # know about — e.g. the IaC lane records source_format and severity_fidelity
    # (`kics`/`native` since #315; `asff`/`full` or `sarif`/`critical-folded-into-high`
    # before). A reader has to know which path produced an HDF before trusting
    # its severities.
    for key, value in d.items():
        if key in RESERVED or not value:
            continue
        rec[key] = value
    return rec


def _parse_args() -> argparse.Namespace:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--lane", required=True,
                    help="evidence lane, e.g. sca-source or iac")
    ap.add_argument("--out", required=True, help="output JSON path")
    ap.add_argument("--scanner", action="append", default=[],
                    help="k=v,... — repeatable, one per scanner")
    ap.add_argument("--gate", default="", help="k=v,... describing the gate outcome")
    ap.add_argument("--emit", default="", help="k=v,... describing the emit outcome")
    ap.add_argument("--scan-path", default="", help="subtree that was scanned")
    ap.add_argument("--now", default="", help="override timestamp (tests)")
    ap.add_argument("--require-hdf", action="store_true",
                    help="exit 1 if a scanner reports ok without a readable HDF")
    return ap.parse_args()


def _env(name: str):
    """An Actions context variable, or None when unset or empty."""
    return os.environ.get(name) or None


def _optional_kv(spec: str, what: str):
    return _kv(spec, what) if spec else None


def _build_doc(args: argparse.Namespace, scanners: list) -> dict:
    now = args.now or _dt.datetime.now(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    return {
        "schema": SCHEMA,
        "lane": args.lane,
        "generated_at": now,
        "repo": _env("GITHUB_REPOSITORY"),
        "commit": _env("GITHUB_SHA"),
        "ref": _env("GITHUB_REF"),
        "run_id": _env("GITHUB_RUN_ID"),
        "run_attempt": _env("GITHUB_RUN_ATTEMPT"),
        "workflow": _env("GITHUB_WORKFLOW"),
        "scan_path": args.scan_path or None,
        "scanners": scanners,
        "gate": _optional_kv(args.gate, "gate"),
        "emit": _optional_kv(args.emit, "emit"),
    }


def _write_doc(path: str, doc: dict) -> None:
    out = _confined(path)
    Path(out).parent.mkdir(parents=True, exist_ok=True)
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(doc, fh, indent=2, sort_keys=False)
        fh.write("\n")


def _trace_line(s: dict) -> str:
    """One scanner's log line. The point is that "0 findings" and "we don't
    know" read differently in the log, not just in the file."""
    if s["status"] == "ok":
        n = s["findings"]
        detail = f"{n} finding(s)" if n is not None else "count unavailable"
        return f"  {s['name']}: ok — {detail}"
    suffix = f" — {s['error']}" if s.get("error") else ""
    return f"  {s['name']}: {s['status']}{suffix}"


def _missing_hdf(scanners: list) -> list:
    return [s["name"] for s in scanners
            if s["status"] == "ok" and not (s["hdf"] and s["hdf_sha256"])]


def main() -> int:
    args = _parse_args()
    scanners = [build_scanner(s) for s in args.scanner]
    _write_doc(args.out, _build_doc(args, scanners))

    for s in scanners:
        print(_trace_line(s))
    print(f"provenance written to {args.out} (lane={args.lane}, scanners={len(scanners)})")

    if args.require_hdf:
        bad = _missing_hdf(scanners)
        if bad:
            die("scanner(s) reported ok with no readable HDF: " + ", ".join(bad))

    return 0


if __name__ == "__main__":
    sys.exit(main())
