#!/usr/bin/env bash
#
# Fail unless every file the build container downloads is verified against a
# SHA-256 pinned for the version mise.toml pins.
#
# Usage:
#   scripts/check-tool-checksums.sh
#
# Run from the repository root. build/tool-checksums.txt holds one line per
# tool, version and architecture, "<tool> <version> <amd64|arm64> <sha256>",
# and build/Dockerfile checks each download against it with
# scripts/verify-download.sh. This holds that file to mise.toml: each tool the
# image downloads is pinned at mise.toml's version for both architectures, and
# no line names anything else. So a version bump without its checksums fails
# `task lint`, not first the container build, and a stale line cannot linger.
# It also counts as many verify-download calls in the Dockerfile as downloads.
set -euo pipefail

if (($# != 0)); then
  sed -n '6,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

readonly checksums="build/tool-checksums.txt"
readonly dockerfile="build/Dockerfile"
readonly mise_toml="mise.toml"
readonly downloaded="node shellcheck jq lefthook typos hadolint taplo zizmor"
readonly arches="amd64 arm64"

for source in "${checksums}" "${dockerfile}" "${mise_toml}"; do
  if [[ ! -f "${source}" ]]; then
    echo "check-tool-checksums: cannot read ${source}" >&2
    exit 1
  fi
done

failed=0

# problem reports one gap and marks the run failed.
problem() {
  echo "check-tool-checksums: $*" >&2
  failed=1
}

# pin prints mise.toml's version of a tool, or nothing.
pin() {
  sed -n -E "s/^\"?$1\"?[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\1/p" "${mise_toml}" | head -1
}

# pins_of prints how many lines pin a tool, version and architecture.
pins_of() {
  awk -v key="$1" '$1 " " $2 " " $3 == key' "${checksums}" | grep -c . || true
}

wanted="|"
for tool in ${downloaded}; do
  version="$(pin "${tool}")"
  if [[ -z "${version}" ]]; then
    problem "${mise_toml} pins no version of ${tool}"
    continue
  fi
  for arch in ${arches}; do
    key="${tool} ${version} ${arch}"
    wanted="${wanted}${key}|"
    count="$(pins_of "${key}")"
    if ((count == 0)); then
      problem "no SHA-256 for ${key} in ${checksums}: add the sha256sum of the file the Dockerfile downloads"
    elif ((count > 1)); then
      problem "${key} is pinned ${count} times in ${checksums}"
    fi
  done
done

while read -r tool version arch sum extra; do
  if [[ -z "${tool}" || "${tool}" == \#* ]]; then
    continue
  fi
  key="${tool} ${version} ${arch}"
  if [[ "${wanted}" != *"|${key}|"* ]]; then
    problem "${checksums}: ${key} is not mise.toml's version of a tool the build container downloads"
  fi
  if [[ ! "${sum}" =~ ^[0-9a-f]{64}$ || -n "${extra}" ]]; then
    problem "${checksums}: ${key} is pinned to '${sum}${extra:+ ${extra}}', not a SHA-256"
  fi
done <"${checksums}"

downloads="$(grep -v '^[[:space:]]*#' "${dockerfile}" | grep -c 'curl ' || true)"
verifications="$(grep -v '^[[:space:]]*#' "${dockerfile}" | grep -c 'verify-download ' || true)"
if ((downloads != verifications)); then
  problem "${dockerfile} makes ${downloads} downloads but ${verifications} verification calls: check each download with verify-download"
fi

if ((failed)); then
  exit 1
fi
echo "check-tool-checksums: every download build/Dockerfile makes is pinned at mise.toml's version."
