#!/usr/bin/env bash
#
# Tests for verify-download.sh: a downloaded file passes only when it matches
# the one SHA-256 the checksums file pins for its tool, version and
# architecture, and fails when it differs, when nothing is pinned for it, when
# two lines pin it, or when the checksums file is missing. It runs on a host
# with GNU's sha256sum and on one with only shasum, as stock macOS is.
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
# The SHA-256 of that text, written out so the test runs no hashing tool itself.
readonly sum="65f4209335494c311ba8fb48b8cd23d4c5bf9269968ef3c018d5ccc3559b17f5"
readonly other="0000000000000000000000000000000000000000000000000000000000000000"

# A PATH with shasum but no sha256sum, holding only what else the script runs.
readonly shasum_only="${workdir}/shasum-only"
mkdir "${shasum_only}"
for tool in awk grep shasum; do
  ln -s "$(command -v "${tool}")" "${shasum_only}/${tool}"
done

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

expect_exit fail "a download that is not there" \
  "${verify}" "${pinned}" "${workdir}/missing.tar.xz" node 24.21.0 amd64

expect_exit pass "a match on a host with only shasum" \
  env PATH="${shasum_only}" "${BASH}" "${verify}" "${pinned}" "${download}" node 24.21.0 amd64

expect_output fail "a mismatch on a host with only shasum" "does not match" \
  env PATH="${shasum_only}" "${BASH}" "${verify}" "${pinned}" "${download}" node 24.21.0 arm64

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
