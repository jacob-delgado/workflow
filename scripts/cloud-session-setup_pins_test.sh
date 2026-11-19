#!/usr/bin/env bash
#
# Tests for where scripts/cloud-session-setup.sh reads its pins from: a tool
# mise.toml does not pin stops the setup with 2, by name, though every stamp
# is in place; and the toolchain is stamped with the digest of its pins, read
# as the gate reads a commit's, once a setup from them finishes, the stamp of
# any other pins removed, and with none while an install is under way. The
# stand-ins lib/cloud-session-setup-fixtures.sh writes record what they were
# asked, so nothing is installed for real.
#
# Usage:
#   scripts/cloud-session-setup_pins_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/cloud-session-setup-fixtures.sh
source "${here}/lib/cloud-session-setup-fixtures.sh"

# The setup takes its lock with flock, which util-linux has and stock macOS
# does not; the script is for a Linux cloud session alone.
command -v flock >/dev/null || {
  echo "cloud-session-setup_pins_test.sh: skipped, no flock here"
  exit 0
}

unpinned="${workdir}/unpinned"
repo "${unpinned}"
installed "${unpinned}.prefix"
repin "${unpinned}" '/^jq = /d'
expect_output 2 "a tool mise.toml does not pin" "no pin for 'jq'" setup "${unpinned}"

# The toolchain is stamped with the digest of the pins it was set up from, as
# lib/toolchain-pins.sh reads them for the gate too, which checks a commit's
# against it, and a stamp of another checkout's pins goes, though nothing was
# installed: there is one toolchain, set up from whichever checkout ran the
# setup last.
restamped="${workdir}/restamped"
repo "${restamped}"
installed "${restamped}.prefix"
rm "${restamped}.prefix/stamps/pins@${pins_digest}"
: >"${restamped}.prefix/stamps/pins@another"
expect_exit pass "the toolchain's pins stamp" setup "${restamped}"
count_case
if [[ "$(cd "${restamped}.prefix/stamps" && echo pins@*)" != "pins@${pins_digest}" ]]; then
  fail_case "the toolchain's pins stamp" "it does not name this checkout's pins alone: $(ls "${restamped}.prefix/stamps")"
fi

# A setup stopped part way through an install leaves the toolchain stamped
# with no pins, so the gate runs on none until a setup finishes.
unstamped="${workdir}/unstamped"
repo "${unstamped}"
installed "${unstamped}.prefix"
rm "${unstamped}.prefix/stamps/jq@1.8.2"
expect_exit 1 "a setup stopped part way" setup "${unstamped}"
count_case
if compgen -G "${unstamped}.prefix/stamps/pins@*" >/dev/null; then
  fail_case "a setup stopped part way" "the toolchain is still stamped with pins: $(ls "${unstamped}.prefix/stamps")"
fi

finish_tests
