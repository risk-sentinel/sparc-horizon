#!/usr/bin/env python3
"""Summarise an HDF file: control counts by status and severity.

This is the `saf view summary` step #204 asks for, written in Python rather than
pulling `npx @mitre/saf` into a repo that standardised on the Go hdf CLI. It
needs no network and no npm, so it also runs in the offline selftest.

Reads both shapes the CLI emits:
  native  -> baselines[].requirements[]
  hdf@1   -> profiles[].controls[]

Severity bands follow Heimdall's impact scale:
  0.9-1.0 critical | 0.7-0.8 high | 0.4-0.6 medium | 0.1-0.3 low | 0.0 none

A clean scan legitimately prints all zeros. That is why the caller must ALSO
write a provenance record (provenance.py) — this output alone cannot distinguish
"nothing was wrong" from "nothing was converted".
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from collections import Counter

BANDS = ((0.9, "critical"), (0.7, "high"), (0.4, "medium"), (0.1, "low"))
STATUSES = ("failed", "passed", "skipped", "error", "no_impact")


def die(msg: str) -> None:
    print(f"::error::summary: {msg}", file=sys.stderr)
    sys.exit(1)


def _confined(path: str) -> str:
    """Refuse paths resolving outside CWD (SonarPython S2083)."""
    base = os.path.realpath(os.getcwd())
    target = os.path.realpath(path)
    if base != target and os.path.commonpath([base, target]) != base:
        die(f"refusing to read {path!r} — resolves outside {base}")
    return target


def band(impact) -> str:
    try:
        val = float(impact)
    except (TypeError, ValueError):
        return "unknown"
    for threshold, label in BANDS:
        if val >= threshold:
            return label
    return "none"


def iter_controls(doc: dict):
    """Yield (impact, status) for every control, whichever shape the doc is."""
    groups = doc.get("baselines") or doc.get("profiles") or []
    for group in groups:
        items = group.get("requirements") or group.get("controls") or []
        for item in items:
            yield item.get("impact"), control_status(item)


def control_status(item: dict) -> str:
    """A control's rolled-up status: failed wins, then error, then passed."""
    results = item.get("results") or []
    seen = {r.get("status") for r in results}
    for candidate in ("failed", "error", "passed", "skipped"):
        if candidate in seen:
            return candidate
    return "no_impact"


def tabulate(doc: dict):
    by_status: Counter = Counter()
    failed_by_band: Counter = Counter()
    for impact, status in iter_controls(doc):
        by_status[status] += 1
        if status == "failed":
            failed_by_band[band(impact)] += 1
    return by_status, failed_by_band


def render(name: str, by_status: Counter, failed_by_band: Counter) -> str:
    total = sum(by_status.values())
    lines = [f"### HDF summary — `{name}`", ""]
    if total == 0:
        lines += ["**0 controls.** Either a clean scan or a broken conversion — "
                  "see the provenance record to tell which.", ""]
        return "\n".join(lines)

    lines += ["| status | count |", "| --- | --- |"]
    lines += [f"| {s} | {by_status[s]} |" for s in STATUSES if by_status[s]]
    lines += [f"| **total** | **{total}** |", ""]

    if failed_by_band:
        lines += ["Failed controls by severity:", "",
                  "| severity | count |", "| --- | --- |"]
        for label in ("critical", "high", "medium", "low", "none", "unknown"):
            if failed_by_band[label]:
                lines.append(f"| {label} | {failed_by_band[label]} |")
        lines.append("")
    return "\n".join(lines)


def write_github_output(by_status: Counter) -> None:
    path = os.environ.get("GITHUB_OUTPUT")
    if not path:
        return
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(f"findings={by_status['failed']}\n")
        fh.write(f"controls={sum(by_status.values())}\n")


def write_step_summary(text: str) -> None:
    path = os.environ.get("GITHUB_STEP_SUMMARY")
    if not path:
        return
    with open(path, "a", encoding="utf-8") as fh:
        fh.write(text + "\n")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("hdf", help="path to an HDF JSON file")
    ap.add_argument("--label", default="", help="name to show in the heading")
    ap.add_argument("--no-step-summary", action="store_true",
                    help="print only, do not append to $GITHUB_STEP_SUMMARY")
    args = ap.parse_args()

    try:
        with open(_confined(args.hdf), "r", encoding="utf-8") as fh:
            doc = json.load(fh)
    except OSError as exc:
        die(f"cannot read {args.hdf}: {exc}")
    except json.JSONDecodeError as exc:
        die(f"{args.hdf} is not valid JSON: {exc}")

    by_status, failed_by_band = tabulate(doc)
    text = render(args.label or os.path.basename(args.hdf), by_status, failed_by_band)

    print(text)
    if not args.no_step_summary:
        write_step_summary(text)
    write_github_output(by_status)
    return 0


if __name__ == "__main__":
    sys.exit(main())
