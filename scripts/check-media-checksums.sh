#!/usr/bin/env bash
#
# Fail unless docs/media/tool-checksums.txt pins one SHA-256 for ttyd at the
# version mise.toml pins, for each machine scripts/docs-media.sh downloads it
# for, and nothing else.
#
# Usage:
#   scripts/check-media-checksums.sh
#
# scripts/docs-media.sh (`task docs:media`) hashes every ttyd it runs against
# those lines and refuses one with no line, so a ttyd bump in mise.toml that
# left them behind would surface only when someone next regenerated the
# docs' images. This holds the file to mise.toml in `task lint` instead, as
# check-tool-checksums.sh holds build/tool-checksums.txt: ttyd's pin is read
# as the age gate reads it (lib/dependency-pins.sh), inline table and all, and
# the machines from the pattern docs-media.sh matches `uname -m` against, so
# the two cannot drift apart. A line for another tool, version or machine
# fails, and so does a hash that is not 64 lowercase hex digits.
#
# The environment is the seam: MEDIA_CHECKSUMS_ROOT (the repository this
# script is in) is the checkout it reads.
set -euo pipefail

if (($# != 0)); then
  sed -n '7,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here
root="${MEDIA_CHECKSUMS_ROOT:-$(cd "${here}/.." && pwd)}"
readonly root
readonly checksums="docs/media/tool-checksums.txt"
readonly docs_media="scripts/docs-media.sh"

# shellcheck source=lib/dependency-pins.sh
source "${here}/lib/dependency-pins.sh"

for source in "${checksums}" "${docs_media}" mise.toml; do
  if [[ ! -f "${root}/${source}" ]]; then
    echo "check-media-checksums: cannot read ${source}" >&2
    exit 1
  fi
done

failed=0

# problem reports one gap and marks the run failed.
problem() {
  echo "check-media-checksums: $*" >&2
  failed=1
}

version="$(mise_pins <"${root}/mise.toml" | awk '$1 == "ttyd" { print $2; exit }')"
machines="$(sed -n -E 's/.*=~ \^\(([a-z0-9_|]+)\)\$.*/\1/p' "${root}/${docs_media}" | tr '|' ' ')"

wanted="|"
if [[ -z "${version}" ]]; then
  problem "mise.toml pins no version of ttyd"
elif [[ -z "${machines}" ]]; then
  problem "cannot read the machines ${docs_media} downloads ttyd for from its uname -m pattern"
else
  for machine in ${machines}; do
    key="ttyd ${version} ${machine}"
    wanted="${wanted}${key}|"
    count="$(awk -v key="${key}" '$1 " " $2 " " $3 == key' "${root}/${checksums}" | grep -c . || true)"
    if ((count == 0)); then
      problem "no SHA-256 for ${key} in ${checksums}: add the one the release's SHA256SUMS lists"
    elif ((count > 1)); then
      problem "${key} is pinned ${count} times in ${checksums}"
    fi
  done
fi

while read -r tool pinned machine sum extra; do
  if [[ -z "${tool}" || "${tool}" == \#* ]]; then
    continue
  fi
  key="${tool} ${pinned} ${machine}"
  if [[ "${wanted}" != *"|${key}|"* ]]; then
    problem "${checksums}: ${key} is not mise.toml's version of ttyd for a machine ${docs_media} downloads it for"
  fi
  if [[ ! "${sum}" =~ ^[0-9a-f]{64}$ || -n "${extra}" ]]; then
    problem "${checksums}: ${key} is pinned to '${sum}${extra:+ ${extra}}', not a SHA-256"
  fi
done <"${root}/${checksums}"

if ((failed)); then
  exit 1
fi
echo "check-media-checksums: ttyd is pinned at mise.toml's version for every machine ${docs_media} downloads it for."
