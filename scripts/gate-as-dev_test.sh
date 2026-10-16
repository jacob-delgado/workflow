#!/usr/bin/env bash
#
# Tests for scripts/gate-as-dev.sh. As root it refuses a path that is no
# worktree, or a worktree with no commit, waits for any gate run already going,
# checks the disk has room for a race-and-coverage build, clearing the gate
# user's Go build cache with the pinned Go alone when it is short and stopping
# when that is not enough, names the commit for the gate user to fetch, says so
# only when the worktree has uncommitted changes, and hands the run to that user
# with an environment of named variables alone, each of them handed on, and
# without the lock's descriptor. As the gate user it refuses to run without
# task, trusts root's repository once, checks out the commit refs/gate/head
# names, whichever commit it last named, clean but for web/node_modules and
# tmp/, in a checkout of its own and runs the tasks asked for there, one at a
# time, with a temporary directory of its own made fresh, even of a directory a
# test sealed, or, asked for none, the steps of `task check`, leaving out vuln
# only when vuln.go.dev cannot be reached; between steps, a step named twice
# included, but not after the last, it clears its cache when the disk runs short
# and stops when that is not enough, and it exits with the failing step's
# status, with task's own when check's steps cannot be listed, or with git's own
# when it cannot ready its checkout. Stand-ins for df, runuser, curl, task, go
# and id record what they were asked, so no user is switched and no gate runs.
#
# Usage:
#   scripts/gate-as-dev_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly gate="${here}/gate-as-dev.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The gate takes its lock with flock, which util-linux has and stock macOS
# does not; the script is for a Linux cloud session alone.
command -v flock >/dev/null || {
  echo "gate-as-dev_test.sh: skipped, no flock here"
  exit 0
}

readonly stub_bin="${workdir}/stubs"
readonly log="${workdir}/calls.log"
readonly df_queue="${workdir}/df.queue"
readonly handed_files="${workdir}/handed-files"
mkdir -p "${stub_bin}"

# stub writes a stand-in command that logs its name and arguments.
#   stub <name> [body]
stub() {
  printf '#!/usr/bin/env bash\nprintf "%%s %%s\\n" "%s" "$*" >>"%s"\n%s\n' \
    "$1" "${log}" "${2:-}" >"${stub_bin}/$1"
  chmod +x "${stub_bin}/$1"
}

# df reports the next free space queued in ${df_queue}, in GB, and keeps
# reporting the last one once the queue runs out.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub df 'queue="'"${df_queue}"'"
free="$(head -1 "${queue}")"
if (($(wc -l <"${queue}") > 1)); then tail -n +2 "${queue}" >"${queue}.next" && mv "${queue}.next" "${queue}"; fi
printf "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda 1 1 %d 1%% /\n" $((free * 1048576))'
# runuser stands in for switching users, notes the files it was handed open,
# and exits RUNUSER_STATUS.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub runuser 'ls -l "/proc/$$/fd" >>"'"${handed_files}"'" 2>/dev/null
exit "${RUNUSER_STATUS:-0}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub curl 'exit "${CURL_STATUS:-0}"'
stub go
# task notes the temporary directory it was given and the directory it runs
# in, and lists check's steps, or fails to as a Taskfile that does not load
# does, with SUMMARY_STATUS.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub task 'printf "tmpdir %s\npwd %s\n" "${TMPDIR:-}" "${PWD}" >>"'"${log}"'"
if [[ "$*" == "--summary check" ]]; then
  if [[ -n "${SUMMARY_STATUS:-}" ]]; then
    echo "task: Failed to parse Taskfile.yml" >&2
    exit "${SUMMARY_STATUS}"
  fi
  printf "task: check\n\ncommands:\n - Task: lint\n - Task: test:cover\n - Task: vuln\n - Task: vuln:report\n - Task: secrets\n%s" "${EXTRA_STEP:-}"
  exit 0
fi
exit "${TASK_STATUS:-0}"'
# id knows every user but the one MISSING_USER names.
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub id '[[ "$*" == "${MISSING_USER:-}" ]] && exit 1
[[ "$*" == "-un" ]] && echo dev
exit 0'

# repo makes a repository with one commit, which ignores what the real one's
# .gitignore does that a run keeps or clears, and prints nothing.
#   repo <dir>
repo() {
  git init -q "$1"
  printf 'one\n' >"$1/file"
  printf 'coverage.out\nweb/node_modules\ntmp/\n' >"$1/.gitignore"
  git -C "$1" add file .gitignore
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m one
}

# disk queues the free space each df call in a case reports, in GB.
#   disk <GB>...
disk() {
  printf '%s\n' "$@" >"${df_queue}"
}

# The variables root hands the gate user when it has them, as a session sets
# them, each with a value of the test's own.
readonly handed_on=(LANG=C.UTF-8 TERM=dumb
  HTTPS_PROXY=http://proxy.test:3128 https_proxy=http://proxy.test:3128
  HTTP_PROXY=http://proxy.test:3128 http_proxy=http://proxy.test:3128
  NO_PROXY=localhost no_proxy=localhost
  SSL_CERT_FILE=/test/ca.pem NODE_EXTRA_CA_CERTS=/test/ca.pem GIT_SSL_CAINFO=/test/ca.pem
  CURL_CA_BUNDLE=/test/ca.pem REQUESTS_CA_BUNDLE=/test/ca.pem PIP_CERT=/test/ca.pem
  PLAYWRIGHT_BROWSERS_PATH=/test/browsers PLAYWRIGHT_CHROMIUM_EXECUTABLE=/test/chromium)

# as_root runs the gate as root would, with the stand-ins, a lock of its own
# and the variables root hands on.
#   as_root <gate arg>...
as_root() {
  : >"${log}"
  : >"${handed_files}"
  env PATH="${stub_bin}:${PATH}" GATE_TOOLS="${workdir}/tools" GATE_LOCK="${workdir}/gate.lock" \
    "${handed_on[@]}" GITHUB_TOKEN=secret-for-the-test "${gate}" "$@"
}

# as_gate_user runs the gate's second half, as the gate user would, against a
# checkout under a home of its own.
#   as_gate_user <home> <common dir> <sha> [task...]
as_gate_user() {
  local home="$1"
  shift
  : >"${log}"
  mkdir -p "${home}"
  env PATH="${stub_bin}:${PATH}" GATE_TOOLS="${workdir}/tools" \
    HOME="${home}" GIT_CONFIG_GLOBAL="${home}/.gitconfig" "${gate}" --as-gate-user "$@"
}

# ran prints the task runs a case made, but check's summary, one per line.
ran() {
  grep '^task ' "${log}" | grep -v -- '--summary' || true
}

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
for wanted in ZIZMOR_OFFLINE=true GOTOOLCHAIN=local HOME=/home/dev \
  COREPACK_NPM_REGISTRY=https://registry.npmjs.org "PATH=${stub_bin}:${PATH}" "${handed_on[@]}"; do
  count_case
  if [[ " ${handed} " != *" ${wanted} "* ]]; then
    fail_case "the gate user's environment" "it lacks ${wanted}"
  fi
done
count_case
if [[ "${handed}" == *GITHUB_TOKEN* ]]; then
  fail_case "the gate user's environment" "root's GitHub token reached it"
fi
count_case
if [[ "$(git -C "${source_repo}" rev-parse refs/gate/head)" != "${sha}" ]]; then
  fail_case "the commit to fetch" "refs/gate/head does not name HEAD"
fi

disk 9 8
expect_exit pass "short of room until the cache is cleared" as_root "${source_repo}" lint
count_case
if ! grep -qxF "runuser -u dev -- env -i HOME=/home/dev GOCACHE=/home/dev/.cache/go-build GOTOOLCHAIN=local ${workdir}/tools/go/bin/go clean -cache" "${log}" \
  || grep -q '^go ' "${log}"; then
  fail_case "short of room" "dev's build cache was not the one cleared: $(grep 'clean -cache' "${log}" || true)"
fi
# The lock's descriptor is not among the files runuser is handed open. Linux
# lists a process's open files under /proc; elsewhere nothing is noted.
if [[ -d /proc/self/fd ]]; then
  count_case
  if grep -qF "${workdir}/gate.lock" "${handed_files}"; then
    fail_case "the gate's lock" "it was handed the lock, which it would hold past the run"
  fi
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

exec {lock}>"${workdir}/gate.lock"
flock "${lock}"
export GATE_LOCK_WAIT=1
expect_output 1 "another gate run going" "another gate run" as_root "${source_repo}" lint
unset GATE_LOCK_WAIT
exec {lock}>&-

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
