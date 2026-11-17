#!/usr/bin/env bash
# Smoke-test a built sparc-horizon image the way a user runs it.
#
#   tests/container-smoke/smoke.sh <image> [host port]
#
# Used by container.yml on every PR and by build-sign-publish.yml's verify
# jobs before a release can be approved, so the image that is tested before
# publishing is held to exactly the PR checks. Exits non-zero on any failure.
#
# It asserts what the container serves, not that it started: the image runs
# as 1001:1001 with no /bin/sh, /healthz answers, the API answers as a persona,
# the refusal rule holds (404 for an absent node, 401 for an unknown persona),
# every response a persona can reach is readable, and the demos are served.
set -euo pipefail

image="${1:?usage: smoke.sh <image> [host port]}"
port="${2:-8080}"
name="horizon-smoke-$$"
base="http://127.0.0.1:${port}"
work="$(mktemp -d)"
fail=0

trap 'docker rm -f "${name}" > /dev/null 2>&1 || true; rm -rf "${work}"' EXIT

check() {
  local label="$1"
  local want="$2"
  local got="$3"
  if [[ "${got}" = "${want}" ]]; then
    echo "PASS — ${label}: ${got}"
  else
    echo "::error::${label}: ${got}, want ${want}"
    fail=1
  fi
}

# status <body file> <path> [curl args...]: the HTTP status, body kept.
status() {
  local out="$1"
  local path="$2"
  shift 2
  curl -s -o "${out}" -w '%{http_code}' "$@" "${base}${path}" || true
}

check "image user" "1001:1001" "$(docker image inspect "${image}" --format '{{.Config.User}}')"

# Fails for the right reason only: the exec must be refused because /bin/sh is
# absent, not because the image failed to start at all.
if out="$(docker run --rm --entrypoint /bin/sh "${image}" -c true 2>&1)"; then
  echo "::error::/bin/sh ran inside the image."
  fail=1
elif printf '%s' "${out}" | grep -q 'exec: "/bin/sh": stat /bin/sh'; then
  echo "PASS — the image has no /bin/sh."
else
  echo "::error::/bin/sh failed for an unexpected reason: ${out}"
  fail=1
fi

docker run -d --name "${name}" -p "127.0.0.1:${port}:8080" --read-only --cap-drop ALL \
  --security-opt no-new-privileges "${image}" > /dev/null

healthy=""
for _ in $(seq 1 30); do
  if [[ "$(curl -sf "${base}/healthz" 2> /dev/null | jq -r '.status // empty' 2> /dev/null)" = "ok" ]]; then
    healthy=yes
    break
  fi
  sleep 1
done
if [[ -z "${healthy}" ]]; then
  echo "::error::/healthz never answered {\"status\":\"ok\"} within 30 seconds."
  docker logs "${name}" 2>&1 | tail -20
  exit 1
fi
echo "PASS — /healthz answered."

check "/v1/tree as the default persona" 200 "$(status "${work}/tree.json" /v1/tree)"
nodes="$(jq '[.. | objects | select(has("id") and has("nodeType"))] | length' "${work}/tree.json" 2> /dev/null || echo 0)"
echo "tree carries ${nodes} node object(s)"
if [[ "${nodes}" -lt 1 ]]; then
  echo "::error::/v1/tree returned no nodes."
  fail=1
fi

check "a node that does not exist" 404 \
  "$(status /dev/null /v1/nodes/00000000-0000-5000-8000-000000000000/heat)"
check "an unknown persona" 401 "$(status /dev/null /v1/tree -H 'X-Horizon-Persona: nobody')"

# Walk every persona's tree through the image: each node's heat, each drawn
# column's cell, each boundary's export and the documents it links. This reads
# every golden a tester can reach through the image's own file modes, which a
# single request cannot show.
personas="$(dirname "$0")/../../fixtures/api/personas.json"
walked=0
for who in $(jq -r '.[].id' "${personas}"); do
  hdr=(-H "X-Horizon-Persona: ${who}")
  status "${work}/t.json" /v1/tree "${hdr[@]}" > /dev/null
  for node in $(jq -r '.. | objects | select(has("nodeType") and .nodeType != "system") | .id' "${work}/t.json"); do
    got="$(status "${work}/h.json" "/v1/nodes/${node}/heat" "${hdr[@]}")"
    [[ "${got}" = 200 ]] || check "${who} heat ${node}" 200 "${got}"
    for col in $(jq -r '.columns[]?' "${work}/h.json"); do
      got="$(status /dev/null "/v1/nodes/${node}/cell?col=${col}" "${hdr[@]}")"
      [[ "${got}" = 200 ]] || check "${who} cell ${node} ${col}" 200 "${got}"
      walked=$((walked + 1))
    done
    if [[ "$(jq -r '.. | objects | select(.id == "'"${node}"'") | .nodeType' "${work}/t.json")" = boundary ]]; then
      got="$(status "${work}/e.json" "/v1/export/oscal/${node}" "${hdr[@]}")"
      [[ "${got}" = 200 ]] || check "${who} export ${node}" 200 "${got}"
      while read -r href sum; do
        got="$(status "${work}/doc.json" "${href}" "${hdr[@]}")"
        [[ "${got}" = 200 ]] || check "${who} ${href}" 200 "${got}"
        [[ "$(sha256sum "${work}/doc.json" | cut -d' ' -f1)" = "${sum}" ]] || check "${who} ${href} sha256" "${sum}" mismatch
        walked=$((walked + 1))
      done < <(jq -r '.documents[] | "\(.href) \(.sha256)"' "${work}/e.json")
    fi
    walked=$((walked + 1))
  done
done
check "every persona's reachable responses walked" yes "$([[ "${walked}" -gt 0 && "${fail}" -eq 0 ]] && echo yes || echo "no (${walked} walked)")"

check "/ (demo index)" 200 "$(status "${work}/index.html" /)"
if ! grep -qi '<html' "${work}/index.html" 2> /dev/null; then
  echo "::error::/ did not return HTML."
  fail=1
fi
check "/hud.html" 200 "$(status /dev/null /hud.html)"

check "process uid" 1001 "$(docker top "${name}" -eo pid,uid | awk 'NR==2 {print $2}')"

if [[ "${fail}" -ne 0 ]]; then
  docker logs "${name}" 2>&1 | tail -20
fi
exit "${fail}"
