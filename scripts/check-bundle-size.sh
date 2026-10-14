#!/usr/bin/env bash
#
# Fail when a chunk of the built web app is larger than its limit.
#
# Usage:
#   scripts/check-bundle-size.sh <vite-config> <assets-dir>
#
# The limit is the build's chunkSizeWarningLimit, in kB of 1000 bytes as Vite
# counts them, read from the Vite config: the one place it is set. Vite only
# warns past it, and a warning scrolls by; this fails, so a page that grows
# past it is trimmed, or the limit is raised there on purpose. A chunk is a
# JavaScript file in <assets-dir>; a stylesheet, a font or an image is not, as
# Vite reckons, and a chunk exactly at the limit passes, as Vite warns only
# past it. `yarn build` runs this once Vite has written the app.
#
# Exits 0 when every chunk is within the limit, 1 when one is past it, and 2
# when nothing can be held to it: the config sets no one limit, or
# <assets-dir> holds no chunk.
set -euo pipefail

if (($# != 2)); then
  sed -n '5,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi
readonly config="$1" assets="$2"

limit_kb="$(sed -n -E 's/^[[:space:]]*chunkSizeWarningLimit:[[:space:]]*([0-9]+),?[[:space:]]*$/\1/p' \
  "${config}" 2>/dev/null || true)"
if [[ ! "${limit_kb}" =~ ^[0-9]+$ ]]; then
  echo "check-bundle-size: ${config} sets no one chunkSizeWarningLimit" >&2
  exit 2
fi
readonly limit_kb limit_bytes=$((limit_kb * 1000))

shopt -s nullglob
chunks=("${assets}"/*.js)
shopt -u nullglob
if ((${#chunks[@]} == 0)); then
  echo "check-bundle-size: no chunk in ${assets}" >&2
  exit 2
fi

failed=0
for chunk in "${chunks[@]}"; do
  bytes="$(wc -c <"${chunk}")"
  bytes=$((bytes))
  if ((bytes > limit_bytes)); then
    echo "FAIL  ${chunk##*/}: ${bytes} bytes, past the ${limit_kb} kB limit ${config} sets: trim what the page carries, or raise chunkSizeWarningLimit there on purpose" >&2
    failed=1
  else
    echo "ok    ${chunk##*/}: ${bytes} bytes, within the ${limit_kb} kB limit"
  fi
done

exit "${failed}"
