# shellcheck shell=bash
#
# The harness every script test sources: a work directory removed on exit,
# git's repository variables cleared and its configuration the harness's own,
# with automatic maintenance off, the case counters, the two expectations most
# cases are, a reader of git's trace for the maintenance it started, and the
# summary that ends the run.
#
# Sourced, never run, after the test's own `set -euo pipefail`:
#
#   here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   # shellcheck source=lib/testing.sh
#   source "${here}/lib/testing.sh"
#
# Then a case is one of
#
#   expect_exit <want> <name> <command> [arg...]
#   expect_output <want> <name> <text> <command> [arg...]
#
# where <want> is pass (exit 0), fail (any other exit) or an exact status, and
# <text> must appear in what the command printed, as written: a text of two
# lines wants those two lines together. The command runs in a
# subshell, so a `cd` or an `exit` in it stays there; it may be a function of
# the test's own, and `run_in <dir> <command>` runs one from a directory. What
# it printed is kept in the file ${case_output} names, and shown when the case
# fails; its exit status is left in ${case_status}. A case a test checks itself
# calls count_case, and fail_case when its check fails; one that traces git
# reads what maintenance it started with `git_maintenance_started <dir>`. The
# test ends with finish_tests.

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories a test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT
mkdir "${workdir}/.testing"

# Every git a test starts, and every git the script it drives starts, that keeps
# the test's environment reads a global configuration of the harness's own: the
# developer's and the system's are kept out, and so is git's automatic
# maintenance. A git run under `env -i` reads none of it: e2e-server_test.sh's
# own gits, which only read, and e2e-server.sh's, which read a copy of this
# configuration in the home it gives them, and which e2e-server_test.sh's
# expect_no_maintenance traces, since hook-environment_test.sh cannot see them.
# Since git 2.47 a commit, a fetch or a merge, and the receive-pack a push to a
# repository on disk starts, return with `git maintenance run --auto` detached
# from them and still running, which can be writing into ${workdir} as the trap
# above removes it, and an rm that fails there fails the test. That receive-pack
# reads only its own repository's configuration and this one, never a -c the
# pushing git was handed; an older git's runs gc instead, which receive.autoGc
# stops too. The configuration is lib/test-gitconfig, copied so that a test's
# own `git config --global` changes the copy; e2e-server.sh copies it too, and
# internal/gittest writes the same for the Go tests, which a test of its own
# holds to this file.
cp -- "$(dirname "${BASH_SOURCE[0]}")/test-gitconfig" "${workdir}/.testing/gitconfig"
export GIT_CONFIG_GLOBAL="${workdir}/.testing/gitconfig" GIT_CONFIG_NOSYSTEM=1

cases=0
failures=0
# shellcheck disable=SC2034 # read by the tests that source this
case_output=""

# count_case counts one case.
count_case() {
  cases=$((cases + 1))
}

# fail_case records that the case failed, and why.
#   fail_case <name> <reason>
fail_case() {
  echo "FAIL ${1}: ${2}" >&2
  failures=$((failures + 1))
}

# run_in runs a command from a directory.
#   run_in <dir> <command> [arg...]
run_in() {
  cd "$1" || return
  shift
  "$@"
}

# exit_matches says whether an exit status is the one a case wants. A want
# that is none of the three stops the test: a typo must not read as a pass.
#   exit_matches <pass|fail|status> <status>
exit_matches() {
  local want="$1" status="$2"
  case "${want}" in
    pass) ((status == 0)) ;;
    fail) ((status != 0)) ;;
    "" | *[!0-9]*)
      echo "testing.sh: want pass, fail or an exit status, got '${want}'" >&2
      exit 2
      ;;
    *) ((status == want)) ;;
  esac
}

# run_case runs a case's command in a subshell, its output into the case's
# file, and leaves its exit status in case_status.
#   run_case <command> [arg...]
run_case() {
  count_case
  case_output="${workdir}/.testing/case${cases}.out"
  case_status=0
  ("$@") >"${case_output}" 2>&1 || case_status=$?
}

# git_maintenance_started prints every automatic maintenance or gc started by
# the gits that traced their events into a directory (GIT_TRACE2_EVENT naming
# it), each as its command line, sorted, and nothing when none was. A directory
# it cannot read stops the test: a trace never read must not read as a pass.
#   git_maintenance_started <trace dir>
git_maintenance_started() {
  local started status=0
  started="$(grep -rhE '"event":"child_start".*"argv":\["[^"]*","(maintenance|gc)"' -- "$1")" || status=$?
  if ((status > 1)); then
    echo "testing.sh: could not read git's trace in '$1'" >&2
    exit 2
  fi

  if [[ -n "${started}" ]]; then
    sed -E 's/.*"argv":\[([^]]*)\].*/\1/; s/","/ /g; s/"//g' <<<"${started}" | sort
  fi
}

# show_output prints what the case's command printed, indented under its FAIL.
show_output() {
  sed 's/^/    /' "${case_output}" >&2
}

# expect_exit runs a command and wants its exit status.
#   expect_exit <pass|fail|status> <name> <command> [arg...]
expect_exit() {
  local want="$1" name="$2"
  shift 2
  run_case "$@"

  if ! exit_matches "${want}" "${case_status}"; then
    fail_case "${name}" "want ${want}, got exit ${case_status}:"
    show_output
  fi
}

# expect_output runs a command and wants its exit status and a text in what it
# printed.
#   expect_output <pass|fail|status> <name> <text> <command> [arg...]
expect_output() {
  local want="$1" name="$2" text="$3"
  shift 3
  run_case "$@"

  if ! exit_matches "${want}" "${case_status}" || [[ "$(<"${case_output}")" != *"${text}"* ]]; then
    fail_case "${name}" "want ${want} saying '${text}', got exit ${case_status}:"
    show_output
  fi
}

# finish_tests reports the cases and exits non-zero when any failed, or when
# none ran: a test that checked nothing must not pass on nothing.
finish_tests() {
  local suite
  suite="$(basename "$0" .sh)"

  if ((cases == 0)); then
    echo "${suite}: no case ran." >&2
    exit 1
  fi

  if ((failures > 0)); then
    echo "${suite}: ${failures} of ${cases} case(s) failed." >&2
    exit 1
  fi

  echo "${suite}: ${cases} case(s) passed."
}
