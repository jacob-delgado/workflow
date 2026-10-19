# shellcheck shell=bash
# shellcheck disable=SC2154 # here and workdir are the sourcing test's
#
# The fixtures gate-as-dev.sh's tests share: stand-ins for df, runuser, curl,
# task, go and id that record what they were asked, a repository with one
# commit, the free space each df call reports, and the gate run as root and as
# the gate user. gate-as-dev_test.sh tests the run itself, and
# gate-as-dev_docs_test.sh which tasks a run is handed for a docs-only change.
#
# Sourced, never run, after lib/testing.sh, which sets ${workdir}; gate is the
# script beside the sourcing test's ${here}:
#
#   # shellcheck source=lib/gate-as-dev-fixtures.sh
#   source "${here}/lib/gate-as-dev-fixtures.sh"

readonly gate="${here}/gate-as-dev.sh"

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
# checkout under a home of its own, whose global git configuration starts as a
# copy of the harness's, so its gits leave no maintenance running either.
#   as_gate_user <home> <common dir> <sha> [task...]
as_gate_user() {
  local home="$1"
  shift
  : >"${log}"
  mkdir -p "${home}"
  [[ -e "${home}/.gitconfig" ]] || cp -- "${GIT_CONFIG_GLOBAL}" "${home}/.gitconfig"
  env PATH="${stub_bin}:${PATH}" GATE_TOOLS="${workdir}/tools" \
    HOME="${home}" GIT_CONFIG_GLOBAL="${home}/.gitconfig" "${gate}" --as-gate-user "$@"
}

# ran prints the task runs a case made, but check's summary, one per line.
ran() {
  grep '^task ' "${log}" | grep -v -- '--summary' || true
}
