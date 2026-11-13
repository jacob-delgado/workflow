#!/usr/bin/env bash
#
# Tests for where scripts/cloud-session-setup.sh reads its pins from and how
# it installs the two linters pinned under build/: yamllint is installed again
# from the committed requirements file, into a venv in the prefix, when its
# stamp is gone, the one in use kept until the new one is whole;
# markdownlint-cli2 is installed again by npm ci, with no
# install script, from a copy of the committed package.json and lock, its link
# winning over a node's tool of the same name; each linter is installed again
# when its pin file changes, under a stamp naming the new file alone; and a
# tool mise.toml does not pin, or a pin file under build/ that is missing,
# stops the setup with 2, by name, though every stamp is in place; and the
# toolchain is stamped with the digest of its pins, read as the gate reads a
# commit's, once a setup from them finishes, the stamp of any other pins
# removed, and with none while an install is under way. The stand-ins
# lib/cloud-session-setup-fixtures.sh writes record what they were asked, so
# nothing is installed for real.
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

yamllint_repo="${workdir}/yamllint"
repo "${yamllint_repo}"
installed "${yamllint_repo}.prefix"
rm "${yamllint_repo}.prefix/stamps/yamllint@${requirements_digest}"
mkdir -p "${yamllint_repo}.prefix/venv/bin" "${yamllint_repo}.prefix/yamllint.older/bin"
expect_exit pass "yamllint, installed again" setup "${yamllint_repo}"
count_case
if [[ -e "${yamllint_repo}.prefix/venv" || -e "${yamllint_repo}.prefix/yamllint.older" ]]; then
  fail_case "yamllint, installed again" "the venvs it replaced are still there: $(ls "${yamllint_repo}.prefix")"
fi
count_case
if ! grep -qxF "pip install --quiet --no-cache-dir --disable-pip-version-check -r ${yamllint_repo}/build/yamllint/requirements.txt" "${log}"; then
  fail_case "yamllint, installed again" "yamllint is not installed from its requirements file: $(grep '^pip ' "${log}" || true)"
fi
count_case
linked="$(readlink "${yamllint_repo}.prefix/bin/yamllint")"
if [[ "${linked}" != "${yamllint_repo}.prefix/yamllint."*/bin/yamllint || ! -e "${linked}" ]]; then
  fail_case "yamllint, installed again" "the prefix does not link the venv's yamllint: ${linked}"
fi

# A yamllint installed by a setup that stops at a later step is one the gate
# user can run, though the setup never reaches its last chmod.
stopped_later="${workdir}/stopped-later"
repo "${stopped_later}"
installed "${stopped_later}.prefix"
rm "${stopped_later}.prefix/stamps/yamllint@${requirements_digest}" \
  "${stopped_later}.prefix/stamps/markdownlint-cli2@${lock_digest}+node24.21.0"
expect_exit 1 "yamllint, installed by a setup stopped later" setup "${stopped_later}" NPM_STATUS=1
count_case
venv="$(readlink "${stopped_later}.prefix/bin/yamllint")"
venv="${venv%/bin/yamllint}"
if [[ "$(stat -c %A "${venv}")" != d???r-xr-x ]]; then
  fail_case "yamllint, installed by a setup stopped later" "its venv is not open to the gate user: $(stat -c %A "${venv}")"
fi

# An install of yamllint cut short leaves the one in use linked and whole: a
# venv names its own path, so the new one is built beside it, not in its place.
cut_short="${workdir}/cut-short"
repo "${cut_short}"
installed "${cut_short}.prefix"
mkdir -p "${cut_short}.prefix/venv/bin"
: >"${cut_short}.prefix/venv/bin/yamllint"
ln -s "${cut_short}.prefix/venv/bin/yamllint" "${cut_short}.prefix/bin/yamllint"
printf 'pyyaml==6.0.3 --hash=sha256:bbbb\n' >>"${cut_short}/build/yamllint/requirements.txt"
expect_output 1 "a yamllint install cut short" "yamllint" setup "${cut_short}" PIP_STATUS=1
count_case
if [[ ! -e "${cut_short}.prefix/bin/yamllint" ]]; then
  fail_case "a yamllint install cut short" "the yamllint in use is gone: $(ls -l "${cut_short}.prefix/bin/yamllint")"
fi

# A pin file that changes installs its linter again, under a stamp naming the
# new file, and the old stamp goes.
repinned="${workdir}/repinned"
repo "${repinned}"
installed "${repinned}.prefix"
printf 'pyyaml==6.0.3 --hash=sha256:bbbb\n' >>"${repinned}/build/yamllint/requirements.txt"
printf '{"lockfileVersion": 3, "packages": {}}\n' >"${repinned}/build/markdownlint/package-lock.json"
expect_exit pass "pin files that change" setup "${repinned}"
new_requirements="$(digest_of "${repinned}/build/yamllint/requirements.txt")"
new_lock="$(digest_of "${repinned}/build/markdownlint/package-lock.json")"
for pinned in "yamllint@${new_requirements} pip install" "markdownlint-cli2@${new_lock}+node24.21.0 npm ci"; do
  read -r stamp command <<<"${pinned}"
  count_case
  if ! grep -q "^${command} " "${log}" \
    || [[ "$(cd "${repinned}.prefix/stamps" && echo "${stamp%%@*}"@*)" != "${stamp}" ]]; then
    fail_case "pin files that change" "${stamp%%@*} was not installed again under the new pin file's stamp alone: $(ls "${repinned}.prefix/stamps")"
  fi
done

# markdownlint-cli2 is installed by npm ci from a copy of the lock beside it,
# and linked onto PATH over one an older setup left in node's own bin.
relocked="${workdir}/relocked"
repo "${relocked}"
installed "${relocked}.prefix"
rm "${relocked}.prefix/stamps/markdownlint-cli2@${lock_digest}+node24.21.0"
: >"${relocked}.prefix/node/bin/markdownlint-cli2"
expect_exit pass "markdownlint-cli2, installed again" setup "${relocked}"
count_case
if ! grep -qxF "npm ci --prefix ${relocked}.prefix/markdownlint.new --ignore-scripts --no-fund --no-audit" "${log}"; then
  fail_case "markdownlint-cli2, installed again" "npm ci did not install it with no install script: $(grep '^npm ' "${log}" || true)"
fi
count_case
if ! cmp -s "${relocked}/build/markdownlint/package-lock.json" "${relocked}.prefix/markdownlint/package-lock.json" \
  || ! cmp -s "${relocked}/build/markdownlint/package.json" "${relocked}.prefix/markdownlint/package.json"; then
  fail_case "markdownlint-cli2, installed again" "the prefix does not hold the committed package.json and lock"
fi
count_case
if [[ "$(readlink "${relocked}.bin/markdownlint-cli2")" != "${relocked}.prefix/bin/markdownlint-cli2" ]] \
  || [[ "$(readlink "${relocked}.prefix/bin/markdownlint-cli2")" != "${relocked}.prefix/markdownlint/node_modules/.bin/markdownlint-cli2" ]]; then
  fail_case "markdownlint-cli2, installed again" "PATH does not reach the lock's markdownlint-cli2: $(readlink "${relocked}.bin/markdownlint-cli2")"
fi

unfiled="${workdir}/unfiled"
repo "${unfiled}"
installed "${unfiled}.prefix"
rm "${unfiled}/build/markdownlint/package-lock.json"
expect_output 2 "a pin file under build/ that is missing" "no pin file build/markdownlint/package-lock.json" \
  setup "${unfiled}"

# The stamps stay in place, so nothing installs markdownlint-cli2 and only the
# setup's own check can see package.json is gone.
unasked="${workdir}/unasked"
repo "${unasked}"
installed "${unasked}.prefix"
rm "${unasked}/build/markdownlint/package.json"
expect_output 2 "a package.json under build/ that is missing" "no pin file build/markdownlint/package.json" \
  setup "${unasked}"

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
rm "${unstamped}.prefix/stamps/yamllint@${requirements_digest}"
expect_exit 1 "a setup stopped part way" setup "${unstamped}" PIP_STATUS=1
count_case
if compgen -G "${unstamped}.prefix/stamps/pins@*" >/dev/null; then
  fail_case "a setup stopped part way" "the toolchain is still stamped with pins: $(ls "${unstamped}.prefix/stamps")"
fi

finish_tests
