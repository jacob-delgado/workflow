#!/usr/bin/env bash
#
# Runs every other script test, once, the way a git hook would: git exports
# GIT_DIR to its hooks and to `git rebase --exec` (and GIT_INDEX_FILE to a
# pre-commit), and from a linked worktree it is an absolute path into the
# shared repository, so a script test's `git -C <temp dir>` would then work on
# that repository instead of its own. Each test runs here with that
# environment naming a sentinel repository, which must come out exactly as it
# went in, and its own summary line is printed as it passes. This is what
# `task test:scripts` runs.
#
# Each test also runs with every git it starts that keeps the test's
# environment tracing its events (GIT_TRACE2_EVENT), and without this run's
# own git configuration, as it would run alone, and none of those gits may
# start git's automatic maintenance: since git 2.47 a commit, a fetch or the
# receive-pack of a push leaves it running detached, still able to write into
# the test's work directory as its exit trap removes it, which fails the test.
# The reader of those traces is first held to a trace of its own, so a reader
# that names nothing cannot pass every test. A git a test runs under `env -i`
# drops GIT_TRACE2_EVENT, so this check does not see it: e2e-server_test.sh's
# own gits, which only read, and the ones e2e-server.sh builds its fixture
# with, which is why e2e-server_test.sh traces those itself, through a git
# placed first on PATH (expect_no_maintenance).
#
# Usage:
#   scripts/hook-environment_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly sentinel="${workdir}/sentinel"
git init -q "${sentinel}"
cp -R "${sentinel}" "${workdir}/before"

readonly traces="${workdir}/traces"

# The reader the loop below relies on must name what a trace started, or a
# suite whose gits left maintenance running would pass: a trace of three
# children, two of them maintenance and gc.
readonly canned="${workdir}/canned"
mkdir "${canned}"
{
  printf '{"event":"child_start","child_id":0,"child_class":"?","use_shell":false,"argv":["git","maintenance","run","--auto","--detach"]}\n'
  printf '{"event":"child_start","child_id":1,"child_class":"?","use_shell":false,"argv":["git","pack-objects","--all"]}\n'
  printf '{"event":"child_start","child_id":2,"child_class":"?","use_shell":false,"argv":["git","gc","--auto"]}\n'
} >"${canned}/trace"
count_case
started="$(git_maintenance_started "${canned}")"
if [[ "${started}" != $'git gc --auto\ngit maintenance run --auto --detach' ]]; then
  fail_case "the trace reader names each maintenance and gc started" "got '${started}'"
fi

expect_exit 2 "the trace reader stops on a directory it cannot read" \
  git_maintenance_started "${workdir}/missing"

for test in "${here}"/*_test.sh "${here}"/release/*_test.sh; do
  if [[ "${test}" == "${here}/hook-environment_test.sh" ]]; then
    continue
  fi

  suite="${test#"${here}/"}"
  trace="${traces}/${suite}"
  mkdir -p "${trace}"

  expect_exit pass "${suite} with a hook's environment" \
    env -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_NOSYSTEM GIT_TRACE2_EVENT="${trace}" \
    GIT_DIR="${sentinel}/.git" GIT_INDEX_FILE="${sentinel}/.git/index" "${test}"
  if ((case_status == 0)); then
    tail -n 1 "${case_output}"
  fi

  count_case
  started="$(git_maintenance_started "${trace}")"
  if [[ -n "${started}" ]]; then
    fail_case "${suite} leaves no maintenance running" "its gits started ${started//$'\n'/, }"
  fi
done

expect_exit pass "the repository a hook's environment names is left alone" \
  diff -r "${workdir}/before" "${sentinel}"

# A check that read no trace at all would pass whatever the tests' gits did.
count_case
if [[ -z "$(find "${traces}" -type f -print -quit)" ]]; then
  fail_case "the tests' gits are traced" "no git wrote its events under ${traces}"
fi

finish_tests
