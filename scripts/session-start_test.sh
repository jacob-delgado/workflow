#!/usr/bin/env bash
#
# Tests for .claude/hooks/session-start.sh, the SessionStart hook that sets a
# Claude Code cloud session's toolchain up: outside a cloud session it does
# nothing; in one it runs scripts/cloud-session-setup.sh with all its output
# kept in a log, in ~/.cache unless told otherwise, says in one line that the
# toolchain is ready, or that the setup failed or ran out of time and where
# the log is, and never fails the session's start, with an environment file
# or without one; its time limit for the setup is under the one
# .claude/settings.json gives the hook, so that line still reaches the
# session; and it gives the session's commands, once each, the link
# directory on PATH, zizmor offline, corepack's registry and, for Playwright,
# the newest of the image's Chromium headless shells, or its full Chromium
# where it has no shell, quoted as the shell reads it, keeping whatever else
# the environment file holds. The setup it runs is a stand-in.
#
# Usage:
#   scripts/session-start_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly hook="${here}/../.claude/hooks/session-start.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The hook gives the setup a time limit with timeout, which GNU coreutils has
# and stock macOS does not; the hook is for a Linux cloud session alone.
command -v timeout >/dev/null || {
  echo "session-start_test.sh: skipped, no timeout here"
  exit 0
}

# project lays out a checkout with the hook and a stand-in setup that prints a
# line on each stream, leaves a mark, sleeps SETUP_SLEEP seconds and exits
# with SETUP_STATUS.
#   project <dir>
project() {
  mkdir -p "$1/.claude/hooks" "$1/scripts"
  cp "${hook}" "$1/.claude/hooks/session-start.sh"
  cat >"$1/scripts/cloud-session-setup.sh" <<'STUB'
#!/usr/bin/env bash
echo "setup's own output"
echo "setup's own complaint" >&2
: >"$(dirname "$0")/../setup-ran"
sleep "${SETUP_SLEEP:-0}"
exit "${SETUP_STATUS:-0}"
STUB
  chmod +x "$1/scripts/cloud-session-setup.sh"
}

# start runs a checkout's hook as Claude Code would start a session, with a
# log, an environment file and a browsers directory of its own, which holds
# no browser until a case puts one there.
#   start <dir> [VAR=value...]
start() {
  local dir="$1"
  shift
  env CLAUDE_PROJECT_DIR="${dir}" CLOUD_SETUP_LOG="${dir}.log" CLAUDE_ENV_FILE="${dir}.env" \
    CLOUD_SETUP_BROWSERS="${dir}.browsers" "$@" "${dir}/.claude/hooks/session-start.sh"
}

# browsers puts an executable stand-in at each path under a browsers
# directory.
#   browsers <dir> <path>...
browsers() {
  local dir="$1" path
  shift
  for path in "$@"; do
    mkdir -p "$(dirname "${dir}/${path}")"
    install -m 0755 /dev/null "${dir}/${path}"
  done
}

for remote in "" false; do
  local_session="${workdir}/local-${remote:-unset}"
  project "${local_session}"
  expect_exit pass "outside a cloud session (CLAUDE_CODE_REMOTE='${remote}')" \
    start "${local_session}" CLAUDE_CODE_REMOTE="${remote}"
  count_case
  if [[ -e "${local_session}/setup-ran" || -s "${case_output}" || -e "${local_session}.env" ]]; then
    fail_case "outside a cloud session" "the hook ran the setup, printed or wrote to the environment"
  fi
done

ready="${workdir}/ready"
project "${ready}"
printf 'export SOMETHING_ELSE=kept\n' >"${ready}.env"
expect_output pass "a cloud session's setup succeeds" "is ready" start "${ready}" CLAUDE_CODE_REMOTE=true
count_case
if grep -q "setup's own" "${case_output}"; then
  fail_case "a cloud session's setup succeeds" "the setup's own output reached the session"
fi
count_case
if ! grep -q "setup's own output" "${ready}.log" || ! grep -q "setup's own complaint" "${ready}.log"; then
  fail_case "a cloud session's setup succeeds" "the setup's output and errors are not both in the log"
fi
expect_exit pass "a second start of the same session" start "${ready}" CLAUDE_CODE_REMOTE=true
# shellcheck disable=SC2016 # the lines as the hook writes them, unexpanded
for line in 'export SOMETHING_ELSE=kept' 'export PATH="${HOME}/.local/bin:${PATH}"' \
  'export ZIZMOR_OFFLINE=true' 'export COREPACK_NPM_REGISTRY=https://registry.npmjs.org'; do
  count_case
  if [[ "$(grep -cxF "${line}" "${ready}.env")" != 1 ]]; then
    fail_case "the session's environment" "it does not hold ${line} once: $(grep -cxF "${line}" "${ready}.env")"
  fi
done

count_case
if grep -q PLAYWRIGHT_CHROMIUM_EXECUTABLE "${ready}.env"; then
  fail_case "a session with no browsers directory" "Playwright is pointed at a browser that is not there"
fi

# Headless shells in both layouts Playwright has unpacked them in, the full
# Chromium beside them, and revisions a sort by text would put in the wrong
# order.
shells="${workdir}/shells"
project "${shells}"
browsers "${shells}.browsers" chromium \
  chromium_headless_shell-999/chrome-headless-shell-linux64/chrome-headless-shell \
  chromium_headless_shell-1194/chrome-linux/headless_shell \
  chromium_headless_shell-1300/chrome-headless-shell-linux64/chrome-headless-shell
newest="${shells}.browsers/chromium_headless_shell-1300/chrome-headless-shell-linux64/chrome-headless-shell"
expect_exit pass "a session with the image's headless shells" start "${shells}" CLAUDE_CODE_REMOTE=true
expect_exit pass "a second start with the image's headless shells" start "${shells}" CLAUDE_CODE_REMOTE=true
count_case
if [[ "$(grep -cxF "export PLAYWRIGHT_CHROMIUM_EXECUTABLE=${newest}" "${shells}.env")" != 1 ]]; then
  fail_case "a session with the image's headless shells" "Playwright is not pointed at the newest once: $(grep PLAYWRIGHT_CHROMIUM_EXECUTABLE "${shells}.env" || true)"
fi

full="${workdir}/full"
project "${full}"
browsers "${full}.browsers" chromium
expect_exit pass "a session with the image's full Chromium alone" start "${full}" CLAUDE_CODE_REMOTE=true
count_case
if [[ "$(grep -cxF "export PLAYWRIGHT_CHROMIUM_EXECUTABLE=${full}.browsers/chromium" "${full}.env")" != 1 ]]; then
  fail_case "a session with the image's full Chromium alone" "Playwright is not pointed at it: $(grep PLAYWRIGHT_CHROMIUM_EXECUTABLE "${full}.env" || true)"
fi

empty="${workdir}/empty"
project "${empty}"
mkdir -p "${empty}.browsers"
expect_exit pass "a session with no browser" start "${empty}" CLAUDE_CODE_REMOTE=true
count_case
if grep -q PLAYWRIGHT_CHROMIUM_EXECUTABLE "${empty}.env"; then
  fail_case "a session with no browser" "Playwright is pointed at a browser that is not there: $(grep PLAYWRIGHT_CHROMIUM_EXECUTABLE "${empty}.env")"
fi

odd_browsers="${workdir}/an odd; \$HOME dir"
browsers "${odd_browsers}" chromium_headless_shell-1194/chrome-linux/headless_shell
odd_chromium="${odd_browsers}/chromium_headless_shell-1194/chrome-linux/headless_shell"
odd="${workdir}/odd"
project "${odd}"
expect_exit pass "a Chromium at a path the shell would split" \
  start "${odd}" CLAUDE_CODE_REMOTE=true CLOUD_SETUP_BROWSERS="${odd_browsers}"
count_case
# shellcheck disable=SC2016 # expanded by the shell that sources the file
sourced="$(bash -c 'source "$1" 2>&1; printf "%s" "${PLAYWRIGHT_CHROMIUM_EXECUTABLE-}"' _ "${odd}.env")"
if [[ "${sourced}" != "${odd_chromium}" ]]; then
  fail_case "a Chromium at a path the shell would split" "sourcing the environment file gives back: ${sourced}"
fi

no_env_file="${workdir}/no-env-file"
project "${no_env_file}"
expect_output pass "a session without an environment file" "is ready" \
  start "${no_env_file}" CLAUDE_CODE_REMOTE=true CLAUDE_ENV_FILE=

home_log="${workdir}/home-log"
project "${home_log}"
expect_output pass "the log where no other user can write" "${home_log}.home/.cache/cloud-session-setup.log" \
  start "${home_log}" CLAUDE_CODE_REMOTE=true CLOUD_SETUP_LOG= HOME="${home_log}.home"
count_case
if ! grep -qs "setup's own output" "${home_log}.home/.cache/cloud-session-setup.log"; then
  fail_case "the log where no other user can write" "the setup's output is not in ~/.cache"
fi

failed="${workdir}/failed"
project "${failed}"
expect_output pass "a cloud session's setup fails" "failed (exit 3): the session may have its image's tools" \
  start "${failed}" CLAUDE_CODE_REMOTE=true SETUP_STATUS=3
count_case
if ! grep -qF "${failed}.log" "${case_output}"; then
  fail_case "a cloud session's setup fails" "the hook did not say where the log is"
fi

slow="${workdir}/slow"
project "${slow}"
expect_output pass "a setup that runs out of time" "ran out of its 1 s" \
  start "${slow}" CLAUDE_CODE_REMOTE=true SETUP_SLEEP=5 CLOUD_SETUP_TIMEOUT=1

# Claude Code stops the hook at settings.json's timeout, so the setup's own
# limit must run out first for the hook's line to reach the session. A limit
# either file has lost reads as missing, never as 0.
count_case
hook_limit="$(sed -n 's/.*CLOUD_SETUP_TIMEOUT:-\([0-9][0-9]*\).*/\1/p' "${hook}")"
session_limit="$(sed -n 's/.*"timeout": *\([0-9][0-9]*\).*/\1/p' "${here}/../.claude/settings.json")"
if [[ -z "${hook_limit}" || -z "${session_limit}" ]] || ((hook_limit >= session_limit)); then
  fail_case "the setup's time limit" "the hook's ${hook_limit:-missing} s is not under settings.json's ${session_limit:-missing} s"
fi

finish_tests
