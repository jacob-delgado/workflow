#!/usr/bin/env bash
#
# Tests for check-bundle-size.sh: a built app whose every chunk is within the
# limit its Vite config sets passes, one chunk past it fails by name with exit
# 1, and a stylesheet or a font is no chunk. A config that sets no one limit,
# or an assets directory that holds no chunk or is not there, is refused with a
# status of its own, since a check with nothing to hold to the limit must not
# pass; so is a call missing an argument, with its usage.
#
# Usage:
#   scripts/check-bundle-size_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-bundle-size.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# config writes a Vite config whose build sets the limits given, one line
# each, as vite.config.ts spells its own.
#   config <name> [limit...]
config() {
  local file="${workdir}/$1.config.ts" limit
  shift
  {
    printf 'export default defineConfig({\n  build: {\n'
    for limit in "$@"; do
      printf '    chunkSizeWarningLimit: %s,\n' "${limit}"
    done
    printf '  },\n})\n'
  } >"${file}"
  printf '%s' "${file}"
}

# assets makes an assets directory holding a file of each size given, as
# name=bytes.
#   assets <name> [file=bytes...]
assets() {
  local dir="${workdir}/$1" entry
  shift
  mkdir -p "${dir}"
  for entry in "$@"; do
    head -c "${entry#*=}" /dev/zero >"${dir}/${entry%%=*}"
  done
  printf '%s' "${dir}"
}

limit="$(config limit 900)"

within="$(assets within index-small.js=863291 index-small.css=35094)"
expect_output pass "a chunk within the limit" "index-small.js: 863291 bytes, within the 900 kB limit" "${check}" "${limit}" "${within}"

past="$(assets past index-small.js=1000 index-big.js=900001)"
expect_output 1 "a chunk past the limit, named" "index-big.js: 900001 bytes, past the 900 kB limit" \
  "${check}" "${limit}" "${past}"

at_limit="$(assets at-limit index-full.js=900000)"
expect_exit pass "a chunk exactly at the limit, which Vite does not warn of" "${check}" "${limit}" "${at_limit}"

no_chunk_past="$(assets no-chunk-past index-small.js=1000 index.css=900001 face.woff2=900001)"
expect_exit pass "a stylesheet and a font past the limit, which are no chunks" \
  "${check}" "${limit}" "${no_chunk_past}"

unset_limit="$(config unset)"
expect_output 2 "a config that sets no limit" "sets no one chunkSizeWarningLimit" \
  "${check}" "${unset_limit}" "${within}"

two_limits="$(config two 900 800)"
expect_output 2 "a config that sets two limits" "sets no one chunkSizeWarningLimit" \
  "${check}" "${two_limits}" "${within}"

expect_exit 2 "a config that is not there" "${check}" "${workdir}/missing.config.ts" "${within}"

empty="$(assets empty index.css=10)"
expect_output 2 "an assets directory that holds no chunk" "no chunk in" "${check}" "${limit}" "${empty}"

expect_output 2 "an assets directory that is not there" "no chunk in ${workdir}/missing" \
  "${check}" "${limit}" "${workdir}/missing"

expect_output 2 "a call with no assets directory, refused with its usage" \
  "scripts/check-bundle-size.sh <vite-config> <assets-dir>" "${check}" "${limit}"

finish_tests
