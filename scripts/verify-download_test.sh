#!/usr/bin/env bash
#
# Tests for verify-download.sh: a downloaded file passes only when it matches
# the one SHA-256 the checksums file pins for its tool, version and
# architecture, and fails when it differs, when nothing is pinned for it, when
# two lines pin it, or when the checksums file is missing.
#
# Usage:
#   scripts/verify-download_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly verify="${here}/verify-download.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly download="${workdir}/node.tar.xz"
printf 'the archive\n' >"${download}"
sum="$(sha256sum "${download}" | cut -d' ' -f1)"
readonly sum
readonly other="0000000000000000000000000000000000000000000000000000000000000000"

# checksums writes a checksums file of the given lines and prints its path.
#   checksums <name> <line>...
checksums() {
  local file="${workdir}/$1.txt"
  shift
  printf '# pinned SHA-256s\n' >"${file}"
  printf '%s\n' "$@" >>"${file}"
  printf '%s' "${file}"
}

pinned="$(checksums pinned "node 24.21.0 amd64 ${sum}" "node 24.21.0 arm64 ${other}")"
expect_exit pass "a download that matches its pin" \
  "${verify}" "${pinned}" "${download}" node 24.21.0 amd64

expect_output fail "a download that differs from its pin" "does not match" \
  "${verify}" "${pinned}" "${download}" node 24.21.0 arm64

expect_output fail "a version nothing pins" "no pinned SHA-256 for node 24.22.0 amd64" \
  "${verify}" "${pinned}" "${download}" node 24.22.0 amd64

twice="$(checksums twice "node 24.21.0 amd64 ${sum}" "node 24.21.0 amd64 ${sum}")"
expect_output fail "a download pinned twice" "pinned 2 times" \
  "${verify}" "${twice}" "${download}" node 24.21.0 amd64

malformed="$(checksums malformed "node 24.21.0 amd64 not-a-hash")"
expect_output fail "a pin that is not a SHA-256" "not a SHA-256" \
  "${verify}" "${malformed}" "${download}" node 24.21.0 amd64

expect_exit fail "a checksums file that is not there" \
  "${verify}" "${workdir}/missing.txt" "${download}" node 24.21.0 amd64

expect_exit 2 "too few arguments" "${verify}" "${pinned}" "${download}" node 24.21.0

finish_tests
