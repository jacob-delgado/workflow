#!/usr/bin/env bash
#
# Tests for scripts/gate-as-dev.sh. As root it refuses a path that is no
# worktree, or a worktree with no commit, or a commit whose pins are not the
# ones the toolchain was set up from, though not one that moves only what the
# setup installs nothing from, waits for any gate run already going, root's
# half or a gate user's half that outlived it, checks the disk has room for a
# race-and-coverage build, clearing the gate user's Go build cache with
# the pinned Go alone when it is short and stopping when that is not enough,
# names the commit for the gate user to fetch, and the base GATE_BASE names,
# or none where the worktree holds no such base, which it says, both under
# the lock, says so only when the worktree has uncommitted changes, and hands
# the run to that user with an environment of named variables alone, each of
# them handed on, under flock on the run lock, and without either lock's
# descriptor, started in / wherever root started, by whatever path, running
# the text the run began with from a copy under the toolchain's run/, though
# a setup installed another gate over its path meanwhile. As the gate user it
# refuses to run without task, trusts root's repository once, checks out the
# commit refs/gate/head names, whichever commit it last named, clean but for
# web/node_modules and tmp/, in a checkout of its own, with the base handed
# on fetched and named to task, or none, and runs the tasks asked for there,
# one at a time, with a temporary directory of its own made fresh,
# even of a directory a test sealed, or, asked for none, the steps of `task
# check`, leaving out vuln only when vuln.go.dev cannot be reached; between
# steps, a step named twice included, but not after the last, it clears its
# cache when the disk runs short and stops when that is not enough, and it
# exits with the failing step's status, with task's own when check's steps
# cannot be listed, or with git's own when it cannot ready its checkout.
# Stand-ins for df, runuser, curl, task, go and id record what they were asked,
# so no user is switched and no gate runs.
#
# Usage:
#   scripts/gate-as-dev_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The gate takes its lock with flock, which util-linux has and stock macOS
# does not; the script is for a Linux cloud session alone.
command -v flock >/dev/null || {
  echo "gate-as-dev_test.sh: skipped, no flock here"
  exit 0
}

# shellcheck source=lib/gate-as-dev-fixtures.sh
source "${here}/lib/gate-as-dev-fixtures.sh"

# A gate run hands its base on, so a case names its own or none.
unset GATE_BASE

source_repo="${workdir}/source"
repo "${source_repo}"
sha="$(git -C "${source_repo}" rev-parse HEAD)"
common="${source_repo}/.git"

disk 12
expect_output 2 "no worktree named" "Usage: gate-as-dev" as_root

mkdir -p "${workdir}/plain"
expect_output 2 "a path that is no worktree" "is no git worktree" as_root "${workdir}/plain"
count_case
if grep -q "after the run" "${case_output}"; then
  fail_case "a path that is no worktree" "it reported the disk after a run that never started"
fi

git init -q "${workdir}/unborn"
expect_output 2 "a worktree with no commit" "has no commit to gate" as_root "${workdir}/unborn"
count_case
if grep -q "after the run" "${case_output}"; then
  fail_case "a worktree with no commit" "it reported the disk after a run that never started"
fi

disk 10
expect_exit pass "room to spare" as_root "${source_repo}" lint
count_case
if grep -q "uncommitted changes" "${case_output}"; then
  fail_case "room to spare" "a clean tree was said to have uncommitted changes"
fi
count_case
if ! grep -q "10 GB free on / after the run" "${case_output}"; then
  fail_case "room to spare" "the disk was not reported after the run"
fi
count_case
if grep -q "go clean -cache" "${log}"; then
  fail_case "room to spare" "the build cache was cleared at exactly the headroom"
fi
handed="$(grep -- "--as-gate-user" "${log}" || true)"
count_case
if [[ "${handed}" != *" --as-gate-user ${common} ${sha} lint" ]]; then
  fail_case "room to spare" "the run was not handed to dev: ${handed}"
fi
count_case
if [[ "${handed}" != "runuser -u dev -- env -i "* ]]; then
  fail_case "the gate user's environment" "it is not built from nothing: ${handed}"
fi
for wanted in GOTOOLCHAIN=local HOME=/home/dev \
  COREPACK_NPM_REGISTRY=https://registry.npmjs.org "PATH=${stub_bin}:${PATH}" "${handed_on[@]}"; do
  count_case
  if [[ " ${handed} " != *" ${wanted} "* ]]; then
    fail_case "the gate user's environment" "it lacks ${wanted}"
  fi
done
count_case
if [[ "${handed}" == *ZIZMOR* ]]; then
  fail_case "the gate user's environment" "it hands zizmor a setting: ${handed}"
fi
count_case
if [[ "${handed}" == *GITHUB_TOKEN* ]]; then
  fail_case "the gate user's environment" "root's GitHub token reached it"
fi
# The gate user's half holds the run lock itself, through flock, for as long
# as it runs, and closes it to everything it starts.
count_case
if [[ "${handed}" != *" flock -o -w 3600 ${workdir}/gate-run.lock bash "*" --as-gate-user "* ]]; then
  fail_case "the gate user's run lock" "its half does not run under it: ${handed}"
fi
count_case
if [[ "$(git -C "${source_repo}" rev-parse refs/gate/head)" != "${sha}" ]]; then
  fail_case "the commit to fetch" "refs/gate/head does not name HEAD"
fi

# The base a change is judged against is root's to name: the gate user's
# checkout has only the origin/main of the day it was cloned. So the commit
# GATE_BASE names is named for the gate user to fetch too, and handed on,
# and a base the worktree does not hold is handed on as none, which the
# condition gate says it cannot judge against.
count_case
if [[ " ${handed} " != *" GATE_BASE= "* ]] || ! grep -q "holds no origin/main" "${case_output}"; then
  fail_case "a base the worktree does not hold" "it was not handed on as none: ${handed}"
fi
disk 10
GATE_BASE=HEAD expect_exit pass "a base the worktree holds" as_root "${source_repo}" lint
handed="$(grep -- "--as-gate-user" "${log}" || true)"
count_case
if [[ " ${handed} " != *" GATE_BASE=refs/gate/base "* ]] \
  || [[ "$(git -C "${source_repo}" rev-parse refs/gate/base)" != "${sha}" ]]; then
  fail_case "a base the worktree holds" "refs/gate/base was not named and handed on: ${handed}"
fi

# A setup run while a gate run waits installs another gate over the path root
# started from; the gate user's half still runs the text root's half began
# with, from a copy under the toolchain's run/ that the gate user can read,
# whatever root's umask.
installed="${workdir}/installed-gate/gate-as-dev"
mkdir -p "${installed%/*}/lib"
cp "${gate}" "${installed}"
cp "${here}/lib/toolchain-pins.sh" "${installed%/*}/lib/toolchain-pins.sh"
rm -rf "${workdir}/tools/run"
# with_umask runs a command under a umask; a case runs in a subshell of its
# own, so the umask ends with it.
#   with_umask <mask> <command> [arg...]
with_umask() {
  umask "$1"
  shift
  "$@"
}
disk 12
DF_REPLACE="${installed}" expect_exit pass "a gate installed while the run waits" \
  with_umask 077 run_gate "${installed}" "${source_repo}" lint
handed_copy="$(sed -n 's/.* flock -o -w 3600 [^ ]* bash \([^ ]*\) --as-gate-user .*/\1/p' "${log}")"
count_case
if ! grep -qx "# replaced" "${installed}"; then
  fail_case "a gate installed while the run waits" "no other gate was installed while it waited"
fi
count_case
if [[ ! -f "${handed_copy}" ]] || grep -qx "# replaced" "${handed_copy}" || ! cmp -s "${gate}" "${handed_copy}"; then
  fail_case "a gate installed while the run waits" "the gate user was not handed the text the run began with: ${handed_copy}"
fi
count_case
if [[ "${handed_copy}" != "${workdir}/tools/run/"* || "$(stat -c %a "${handed_copy}" 2>/dev/null)" != 644 ]]; then
  fail_case "the gate user's run copy" "it is not a 0644 file under the toolchain's run/: ${handed_copy}"
fi
count_case
if [[ "$(stat -c %a "${workdir}/tools/run" 2>/dev/null)" != 755 ]]; then
  fail_case "the gate user's run copy" "its directory is not 0755 under umask 077: $(stat -c %a "${workdir}/tools/run" 2>&1)"
fi

disk 9 8
expect_exit pass "short of room until the cache is cleared" as_root "${source_repo}" lint
count_case
if ! grep -qxF "runuser -u dev -- env -i HOME=/home/dev GOCACHE=/home/dev/.cache/go-build GOTOOLCHAIN=local ${workdir}/tools/go/bin/go clean -cache" "${log}" \
  || grep -q '^go ' "${log}"; then
  fail_case "short of room" "dev's build cache was not the one cleared: $(grep 'clean -cache' "${log}" || true)"
fi
# The locks' descriptors are not among the files runuser is handed open.
# Linux lists a process's open files under /proc; elsewhere nothing is noted.
if [[ -d /proc/self/fd ]]; then
  count_case
  if grep -qF -e "${workdir}/gate.lock" -e "${workdir}/gate-run.lock" "${handed_files}"; then
    fail_case "the gate's lock" "it was handed a lock, which it would hold past the run"
  fi
fi

# runuser keeps the directory it is started in, which the gate user cannot
# read when it is root's own, as a worktree under /tmp/claude-0 is. The gate
# is started by a relative path there, as `bash scripts/gate-as-dev.sh` from
# a worktree is, and still finds its library.
mkdir -m 0700 "${workdir}/root-only"
ln -s "${here}" "${workdir}/root-only/scripts"
disk 9 8
expect_exit pass "a run started in a directory of root's own" \
  run_in "${workdir}/root-only" run_gate scripts/gate-as-dev.sh ../source lint
count_case
if [[ "$(cat "${handed_dirs}")" != $'/\n/' ]] || ! grep -q -- " --as-gate-user ${common} ${sha} lint$" "${log}"; then
  fail_case "a run started in a directory of root's own" \
    "the gate user did not start in / on ../source: $(tr '\n' ' ' <"${handed_dirs}")"
fi

disk 5 7
expect_output 1 "short of room even so" "still only 7 GB free" as_root "${source_repo}" lint
count_case
if grep -q -- "--as-gate-user" "${log}"; then
  fail_case "short of room even so" "the gate ran anyway"
fi
count_case
if grep -q "after the run" "${case_output}"; then
  fail_case "short of room even so" "it reported the disk after a run that never started"
fi

disk 12
export RUNUSER_STATUS=7
expect_exit 7 "a failing gate's status" as_root "${source_repo}" lint
unset RUNUSER_STATUS

export MISSING_USER=dev
expect_output 2 "no gate user" "no user dev" as_root "${source_repo}" lint
unset MISSING_USER

# A commit whose pins are not the ones the toolchain was set up from, as a
# lane's that bumps a tool, or one that no longer has its mise.toml.
repinned="${workdir}/repinned"
repo "${repinned}"
printf 'node = "24.22.0"\n' >>"${repinned}/mise.toml"
git -C "${repinned}" -c user.email=t@example.com -c user.name=t commit -q -am repin
expect_output 2 "a toolchain set up from other pins" "run setup for this worktree" as_root "${repinned}" lint
count_case
if grep -q -- "--as-gate-user" "${log}"; then
  fail_case "a toolchain set up from other pins" "the gate ran anyway"
fi
count_case
if grep -q "after the run" "${case_output}"; then
  fail_case "a toolchain set up from other pins" "it reported the disk after a run that never started"
fi
# A commit that moves only what the setup installs nothing from, a tool
# mise.toml pins that the setup leaves alone and the file's other lines, as a
# Dependabot bump of one does, is gated with the toolchain its base set up.
unmoved="${workdir}/unmoved"
repo "${unmoved}"
printf '# a comment\n[env]\nGOFLAGS = "-mod=mod"\n[tools]\nhugo-extended = "0.168.0"\n' >>"${unmoved}/mise.toml"
git -C "${unmoved}" -c user.email=t@example.com -c user.name=t commit -q -am bump
expect_exit pass "a commit moving no pin the setup installs from" as_root "${unmoved}" lint

git -C "${repinned}" rm -q mise.toml
git -C "${repinned}" -c user.email=t@example.com -c user.name=t commit -q -m unpin
expect_output 2 "a commit with its pins gone" "run setup for this worktree" as_root "${repinned}" lint

# A run that waits on another names no base until it holds the lock, so the
# base the run going has yet to fetch stays as that run named it, though this
# one would take it back.
git -C "${source_repo}" update-ref refs/gate/base "${sha}"
exec {lock}>"${workdir}/gate.lock"
flock "${lock}"
export GATE_LOCK_WAIT=1
expect_output 1 "another gate run going" "another gate run" as_root "${source_repo}" lint
unset GATE_LOCK_WAIT
exec {lock}>&-
count_case
if [[ "$(git -C "${source_repo}" rev-parse --verify --quiet refs/gate/base)" != "${sha}" ]]; then
  fail_case "another gate run going" "it moved the base the run going names before it held the lock"
fi

# The gate user's half of a run whose root half was killed holds the run
# lock on, still using its checkout, so the next run waits for it too.
exec {lock}>"${workdir}/gate-run.lock"
flock "${lock}"
export GATE_LOCK_WAIT=1
expect_output 1 "a gate user's half still going" "another gate run" as_root "${source_repo}" lint
unset GATE_LOCK_WAIT
exec {lock}>&-
count_case
if grep -q -- "--as-gate-user" "${log}"; then
  fail_case "a gate user's half still going" "a second run was handed over beside it"
fi

# The lock is taken here first, so the gate is sure to find it held, and let
# go of a second later.
exec {lock}>"${workdir}/gate.lock"
flock "${lock}"
(sleep 1 && flock -u "${lock}") &
holder=$!
export GATE_LOCK_WAIT=20
expect_output pass "a gate run waits for another" "waiting for another gate run" as_root "${source_repo}" lint
unset GATE_LOCK_WAIT
wait "${holder}"
exec {lock}>&-

printf 'two\n' >>"${source_repo}/file"
expect_output pass "uncommitted changes" "checks HEAD (${sha:0:8}) only" as_root "${source_repo}" lint
git -C "${source_repo}" checkout -q -- file

no_task_bin="${workdir}/no-task"
mkdir -p "${no_task_bin}"
ln -s "$(command -v bash)" "${no_task_bin}/bash"
ln -s "${stub_bin}/id" "${no_task_bin}/id"
expect_output 2 "no task for the gate user" "finds no task" \
  env PATH="${no_task_bin}" HOME="${workdir}/home-no-task" "${gate}" --as-gate-user "${common}" "${sha}" lint

home="${workdir}/home-given"
disk 12
expect_exit pass "the tasks asked for" as_gate_user "${home}" "${common}" "${sha}" lint secrets
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint task secrets " ]]; then
  fail_case "the tasks asked for" "they were not run one at a time: ${got}"
fi
count_case
if [[ "$(git -C "${home}/gate" rev-parse HEAD)" != "${sha}" ]]; then
  fail_case "the gate user's checkout" "it is not at the commit asked for"
fi
count_case
if [[ "$(grep '^pwd ' "${log}" | sort -u)" != "pwd ${home}/gate" ]]; then
  fail_case "the gate user's checkout" "task did not run in it: $(grep '^pwd ' "${log}" || true)"
fi
count_case
if [[ "$(grep '^base ' "${log}" | sort -u)" != "base " ]]; then
  fail_case "no base handed on" "task was not told of none: $(grep '^base ' "${log}" || true)"
fi

# A base named as hand_over names it.
git -C "${source_repo}" update-ref refs/gate/base "${sha}"
disk 12
GATE_BASE=refs/gate/base expect_exit pass "a base handed on" \
  as_gate_user "${workdir}/home-based" "${common}" "${sha}" lint
count_case
if [[ "$(grep '^base ' "${log}" | sort -u)" != "base refs/gate/base" ]] \
  || [[ "$(git -C "${workdir}/home-based/gate" rev-parse refs/gate/base)" != "${sha}" ]]; then
  fail_case "a base handed on" "it was not fetched and handed to task: $(grep '^base ' "${log}" || true)"
fi

# Root's own tools keep fixed names in /tmp, golangci-lint's lock among them,
# and an earlier run's are left in the gate user's directory as they were.
mkdir -p "${home}/.cache/tmp"
printf 'left behind\n' >"${home}/.cache/tmp/golangci-lint.lock"
expect_exit pass "a temporary directory of its own" as_gate_user "${home}" "${common}" "${sha}" lint
count_case
if [[ "$(grep '^tmpdir ' "${log}" | sort -u)" != "tmpdir ${home}/.cache/tmp" ]]; then
  fail_case "a temporary directory of its own" "task did not run with TMPDIR=${home}/.cache/tmp: $(grep '^tmpdir ' "${log}" || true)"
fi
count_case
if [[ -z "$(find "${home}/.cache/tmp" -maxdepth 0 -perm 0700)" ]]; then
  fail_case "a temporary directory of its own" "it is not the gate user's alone: $(ls -ld "${home}/.cache/tmp")"
fi
count_case
if [[ -e "${home}/.cache/tmp/golangci-lint.lock" ]]; then
  fail_case "a temporary directory of its own" "it keeps what an earlier run left in it"
fi

# A test binary cut short, by its timeout or by the gate being killed, never
# runs the cleanup that opens a directory it sealed, so a run can find one
# left there. Root removes any file, so only another user can tell.
if ((EUID != 0)); then
  sealed="${home}/.cache/tmp/TestSealed"
  mkdir -p "${sealed}/001"
  printf 'left behind\n' >"${sealed}/001/.workflow.json"
  chmod 0500 "${sealed}/001"
  chmod 0100 "${sealed}"
  expect_exit pass "a sealed directory an earlier run left" as_gate_user "${home}" "${common}" "${sha}" lint
  count_case
  if [[ -e "${sealed}" ]]; then
    fail_case "a sealed directory an earlier run left" "it is still there"
    chmod -R u+rwx "${sealed}"
  fi
fi

# A past run leaves files behind, some of them ignored, as coverage.out is;
# the web's dependencies and tmp/ it keeps for the next.
printf 'left behind\n' >"${home}/gate/stray"
printf 'left behind\n' >"${home}/gate/coverage.out"
mkdir -p "${home}/gate/web/node_modules" "${home}/gate/tmp"
printf 'kept\n' >"${home}/gate/web/node_modules/kept"
printf 'kept\n' >"${home}/gate/tmp/kept"
printf 'changed\n' >"${home}/gate/file"
expect_exit pass "a checkout left dirty" as_gate_user "${home}" "${common}" "${sha}" lint
count_case
if [[ -e "${home}/gate/stray" || -e "${home}/gate/coverage.out" || "$(cat "${home}/gate/file")" != one ]]; then
  fail_case "a checkout left dirty" "the run did not start from the commit's own tree"
fi
count_case
if [[ ! -e "${home}/gate/web/node_modules/kept" || ! -e "${home}/gate/tmp/kept" ]]; then
  fail_case "a checkout left dirty" "it cleared web/node_modules or tmp/, which a run keeps"
fi
count_case
if [[ "$(git config --file "${home}/.gitconfig" --get-all safe.directory)" != "${common}" ]]; then
  fail_case "root's repository" "the gate user does not trust it once: $(git config --file "${home}/.gitconfig" --get-all safe.directory || true)"
fi

export TASK_STATUS=201
expect_exit 201 "a failing step" as_gate_user "${home}" "${common}" "${sha}" lint secrets
unset TASK_STATUS
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint " ]]; then
  fail_case "a failing step" "the steps after it ran: ${got}"
fi

disk 2
expect_exit pass "short between steps until the cache is cleared" \
  as_gate_user "${home}" "${common}" "${sha}" lint secrets
count_case
if ! grep -qx "go clean -cache" "${log}"; then
  fail_case "short between steps" "the build cache was not cleared"
fi
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint task secrets " ]]; then
  fail_case "short between steps until the cache is cleared" "the next step did not run: ${got}"
fi

disk 3
expect_exit pass "exactly the room between steps" \
  as_gate_user "${home}" "${common}" "${sha}" lint secrets
count_case
if grep -q "go clean -cache" "${log}"; then
  fail_case "exactly the room between steps" "the build cache was cleared"
fi

disk 1
expect_output 1 "the disk runs out between steps" "1 GB free after lint" \
  as_gate_user "${home}" "${common}" "${sha}" lint secrets
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint " ]]; then
  fail_case "the disk runs out between steps" "the next step ran: ${got}"
fi

expect_output 1 "a step named again" "1 GB free after lint" \
  as_gate_user "${home}" "${common}" "${sha}" lint secrets lint
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint " ]]; then
  fail_case "a step named again" "the disk was not checked before the next step: ${got}"
fi

expect_exit pass "short after the last step" as_gate_user "${home}" "${common}" "${sha}" lint

disk 12
home="${workdir}/home-online"
expect_exit pass "check, with vuln.go.dev reachable" as_gate_user "${home}" "${common}" "${sha}"
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint task test:cover task vuln task vuln:report task secrets " ]]; then
  fail_case "check, with vuln.go.dev reachable" "check's steps were not run one at a time: ${got}"
fi

home="${workdir}/home-offline"
export CURL_STATUS=56
expect_output pass "check, with vuln.go.dev refused" "vuln.go.dev" as_gate_user "${home}" "${common}" "${sha}"
got="$(ran | tr '\n' ' ')"
count_case
if [[ "${got}" != "task lint task test:cover task vuln:report task secrets " ]]; then
  fail_case "check, with vuln.go.dev refused" "check's other steps were not run: ${got}"
fi

export EXTRA_STEP=' - echo done
'
expect_output 2 "a step of check this cannot run alone" "echo done" \
  as_gate_user "${home}" "${common}" "${sha}"
unset EXTRA_STEP

export SUMMARY_STATUS=104
expect_output 104 "check's steps that cannot be listed" "Failed to parse Taskfile.yml" \
  as_gate_user "${home}" "${common}" "${sha}"
unset SUMMARY_STATUS CURL_STATUS
count_case
if grep -q "no task of its own" "${case_output}"; then
  fail_case "check's steps that cannot be listed" "it blamed a step of check for the Taskfile"
fi

expect_output 128 "a repository the gate user cannot read" "does not exist" \
  as_gate_user "${workdir}/home-no-repo" "${workdir}/no-such-repo/.git" "${sha}" lint
got="$(ran | tr '\n' ' ')"
count_case
if [[ -n "${got}" ]]; then
  fail_case "a repository the gate user cannot read" "a step ran: ${got}"
fi

# Another worktree's HEAD, no descendant of the commit last gated, moves
# refs/gate/head as hand_over would.
other_sha="$(git -C "${source_repo}" -c user.email=t@example.com -c user.name=t \
  commit-tree -m other "${sha}^{tree}")"
git -C "${source_repo}" update-ref refs/gate/head "${other_sha}"
home="${workdir}/home-given"
disk 12
expect_exit pass "a commit that is no descendant of the last" \
  as_gate_user "${home}" "${common}" "${other_sha}" lint
count_case
if [[ "$(git -C "${home}/gate" rev-parse HEAD)" != "${other_sha}" ]]; then
  fail_case "a commit that is no descendant of the last" "the checkout is not at it"
fi

finish_tests
