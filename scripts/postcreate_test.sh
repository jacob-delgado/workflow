#!/usr/bin/env bash
#
# Tests for .devcontainer/postCreate.sh's last step: it hands the rest of the
# setup to `task setup`, in a clone and outside a repository alike, and runs
# none of that task's steps itself, so a step the task gains reaches the dev
# container with no copy here. `task setup` decides whether there is a
# repository to install the git hooks in. Before it, a container whose Python
# has no venv, which `task tools` builds yamllint's in, gets python3-venv from
# apt, and one whose Python has it asks apt for nothing. Stand-ins for mise,
# python3 and sudo record what they were asked and do nothing, so no toolchain
# or package is installed.
#
# Usage:
#   scripts/postcreate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly postcreate="${here}/../.devcontainer/postCreate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly stub_bin="${workdir}/bin"
mkdir -p "${stub_bin}"
cat >"${stub_bin}/mise" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${MISE_STUB_LOG}"
STUB
chmod +x "${stub_bin}/mise"
# python3 answers whether it has ensurepip, which a venv needs, as
# PYTHON3_STATUS says; sudo logs into the same file as mise, so the order of
# the two shows.
cat >"${stub_bin}/python3" <<'STUB'
#!/usr/bin/env bash
exit "${PYTHON3_STATUS:-0}"
STUB
cat >"${stub_bin}/sudo" <<'STUB'
#!/usr/bin/env bash
printf 'sudo %s\n' "$*" >>"${MISE_STUB_LOG}"
STUB
chmod +x "${stub_bin}/python3" "${stub_bin}/sudo"

# workspace copies the script into a fresh workspace, with the mise.toml it
# trusts, and prints its path.
#   workspace <dir>
workspace() {
  mkdir -p "$1/.devcontainer"
  cp "${postcreate}" "$1/.devcontainer/postCreate.sh"
  printf 'min_version = "2026.9.3"\n' >"$1/mise.toml"
}

# commit makes a repository of a workspace, with its files committed.
#   commit <dir>
commit() {
  git -C "$1" init -q
  git -C "$1" add -A
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m workspace
}

# provision runs a workspace's postCreate.sh with the stand-ins and a home of
# its own.
#   provision <dir> [VAR=value...]
provision() {
  local dir="$1"
  shift
  mkdir -p "${dir}.home"
  env PATH="${stub_bin}:${PATH}" HOME="${dir}.home" MISE_STUB_LOG="${dir}.mise" "$@" \
    "${dir}/.devcontainer/postCreate.sh"
}

# What the stand-in mise is asked when postCreate.sh hands over to the
# Taskfile, and the steps of `task setup` it must not run itself.
readonly task_setup='exec -- task setup'
readonly setup_steps='go mod download|lefthook install'

# expect_handed_over checks that a workspace's run asked mise for `task setup`
# and for none of its steps.
#   expect_handed_over <name> <dir>
expect_handed_over() {
  count_case
  if ! grep -qx "${task_setup}" "$2.mise"; then
    fail_case "$1" "mise was not asked to run task setup: $(cat "$2.mise")"
  fi
  count_case
  if grep -qE "${setup_steps}" "$2.mise"; then
    fail_case "$1" "postCreate.sh ran a step of task setup itself: $(cat "$2.mise")"
  fi
}

clone="${workdir}/clone"
workspace "${clone}"
commit "${clone}"
expect_output pass "a clone is set up by task setup" "task setup" provision "${clone}"
expect_handed_over "a clone is set up by task setup" "${clone}"

bare="${workdir}/bare"
workspace "${bare}"
expect_output pass "no repository yet is set up by task setup too" "task setup" provision "${bare}"
expect_handed_over "no repository yet is set up by task setup too" "${bare}"

count_case
if grep -q '^sudo ' "${clone}.mise"; then
  fail_case "a Python with venv" "apt was asked for something: $(grep '^sudo ' "${clone}.mise")"
fi

novenv="${workdir}/novenv"
workspace "${novenv}"
commit "${novenv}"
expect_output pass "a Python with no venv" "python3-venv" provision "${novenv}" PYTHON3_STATUS=1
count_case
apt_line="$(grep -n '^sudo .*apt-get install .*python3-venv' "${novenv}.mise" | cut -d: -f1 || true)"
setup_line="$(grep -nx "${task_setup}" "${novenv}.mise" | cut -d: -f1 || true)"
if [[ -z "${apt_line}" || -z "${setup_line}" ]] || ((apt_line > setup_line)); then
  fail_case "a Python with no venv" "python3-venv was not installed before task setup: $(cat "${novenv}.mise")"
fi

finish_tests
