#!/usr/bin/env bash
#
# Run the gate as the non-root gate user against one commit of a worktree, in
# a checkout that user owns, with the toolchain scripts/cloud-session-setup.sh
# installs.
#
# Usage:
#   scripts/gate-as-dev.sh <worktree> [task...]
#
# A Claude Code cloud session runs as root, and root reads any file: the tests
# that make one unreadable skip under it, and gobco's verdict does not hold
# (CLAUDE.md, Testing). So the run is handed to the gate user (GATE_USER, dev),
# which fetches the worktree's HEAD into ~/gate and runs task there, one task
# at a time; given no task, the steps of `task check`. The gate checks what is
# committed: uncommitted changes are left out, and it says so. One run goes at
# a time, since every run shares that checkout; a second waits for the first
# (GATE_LOCK_WAIT seconds, an hour by default).
#
# The gate user gets an environment of named variables alone, so none of
# root's credentials reach the code under test, and not the descriptor the
# lock is held on, so nothing it leaves running holds the lock past the run.
# It gets a temporary directory of its own too, made fresh under ~/.cache, so
# the fixed names tools keep in /tmp are never root's. It reaches the tools
# through root's PATH, whose ~/.local/bin links into the toolchain; /root is
# readable by others in a cloud session.
#
# The session's disk is a fixed allowance that df reports as free space, and a
# race-and-coverage build fills about 7 GB of Go build cache. Under 10 GB free
# the gate user's cache is cleared first, and under 8 GB even so the run does
# not start. Between steps it checks again: under 3 GB the gate user's cache
# is cleared, and under 2 GB even so the run stops before the next step can
# die part way with "no space left on device".
#
# Exits with the failing step's own status, or task's own when it cannot list
# check's steps, or git's own when the gate user cannot ready its checkout
# (trust the repository, clone it, or fetch, check out or clean the commit);
# 1 when the disk has no room or another run holds the lock too long; and 2
# when no worktree is named or the path named is none or has no commit, the
# gate user does not exist or cannot find task, or `task check` has a step that
# is not a task of its own.
set -euo pipefail

readonly gate_user="${GATE_USER:-dev}"
readonly gate_home="/home/${gate_user}"
readonly tools="${GATE_TOOLS:-/opt/workflow-tools}"
readonly lock_file="${GATE_LOCK:-/tmp/gate-as-dev.lock}"
readonly lock_wait="${GATE_LOCK_WAIT:-3600}"
readonly need_gb=10 floor_gb=8 step_need_gb=3 step_floor_gb=2

# The variables the gate user's environment is built from, when root has them:
# the PATH that reaches the toolchain, the locale and terminal, the proxy the
# session reaches the network through and its certificates, and where
# Playwright finds its browsers.
readonly passed_on=(PATH LANG TERM
  HTTPS_PROXY https_proxy HTTP_PROXY http_proxy NO_PROXY no_proxy
  SSL_CERT_FILE NODE_EXTRA_CA_CERTS GIT_SSL_CAINFO CURL_CA_BUNDLE REQUESTS_CA_BUNDLE PIP_CERT
  PLAYWRIGHT_BROWSERS_PATH)

# free_gb prints the whole gigabytes free on /.
free_gb() {
  df -Pk / | awk 'NR == 2 { print int($4 / 1048576) }'
}

# make_room clears the gate user's Go build cache when the disk is short of
# room for a run, and stops when that still leaves too little.
make_room() {
  local free
  free="$(free_gb)"
  echo "gate-as-dev: ${free} GB free on / before the run"
  ((free >= need_gb)) && return 0
  echo "gate-as-dev: under ${need_gb} GB free, so clearing ${gate_user}'s Go build cache" >&2
  runuser -u "${gate_user}" -- env -i HOME="${gate_home}" GOCACHE="${gate_home}/.cache/go-build" \
    GOTOOLCHAIN=local "${tools}/go/bin/go" clean -cache {lock}>&-
  free="$(free_gb)"
  if ((free < floor_gb)); then
    echo "gate-as-dev: still only ${free} GB free; free space before running the gate" >&2
    exit 1
  fi
}

# take_lock waits for any gate run already going, and holds the lock for this
# one until the script exits.
take_lock() {
  exec {lock}>"${lock_file}"
  flock -n "${lock}" && return 0
  echo "gate-as-dev: waiting for another gate run to finish" >&2
  if ! flock -w "${lock_wait}" "${lock}"; then
    echo "gate-as-dev: another gate run has held ${lock_file} for ${lock_wait} s" >&2
    exit 1
  fi
}

# hand_over names the worktree's HEAD for the gate user to fetch and runs the
# rest of this script as that user.
#   hand_over <worktree> [task...]
hand_over() {
  local src common sha self name
  local -a environment
  if ! id "${gate_user}" >/dev/null 2>&1; then
    echo "gate-as-dev: no user ${gate_user}; scripts/cloud-session-setup.sh makes it" >&2
    exit 2
  fi
  if ! src="$(git -C "$1" rev-parse --show-toplevel 2>/dev/null)"; then
    echo "gate-as-dev: $1 is no git worktree" >&2
    exit 2
  fi
  shift
  common="$(git -C "${src}" rev-parse --path-format=absolute --git-common-dir)"
  if ! sha="$(git -C "${src}" rev-parse --verify --quiet HEAD)"; then
    echo "gate-as-dev: ${src} has no commit to gate" >&2
    exit 2
  fi
  self="$(readlink -f "${BASH_SOURCE[0]}")"
  if [[ -n "$(git -C "${src}" status --porcelain)" ]]; then
    echo "gate-as-dev: ${src} has uncommitted changes; the gate checks HEAD (${sha:0:8}) only" >&2
  fi
  take_lock
  make_room
  trap 'echo "gate-as-dev: $(free_gb) GB free on / after the run"' EXIT
  # A detached HEAD is no ref the gate user can fetch, so name the commit.
  git -C "${src}" update-ref refs/gate/head "${sha}"
  for name in "${passed_on[@]}"; do
    if [[ -n "${!name:-}" ]]; then environment+=("${name}=${!name}"); fi
  done
  # Trade-off TRADE-54: zizmor runs offline here, as it would anyway with no
  # GitHub token handed on; its online audits read each third-party action's
  # tags from github.com, which refuses a session's token. CI's zizmor has no
  # token and runs offline too.
  runuser -u "${gate_user}" -- env -i "${environment[@]}" \
    HOME="${gate_home}" USER="${gate_user}" LOGNAME="${gate_user}" \
    GOPATH="${gate_home}/go" GOCACHE="${gate_home}/.cache/go-build" GOFLAGS="" GOTOOLCHAIN=local \
    XDG_CACHE_HOME="${gate_home}/.cache" COREPACK_HOME="${gate_home}/.cache/node/corepack" \
    COREPACK_NPM_REGISTRY=https://registry.npmjs.org COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    NPM_CONFIG_USERCONFIG="${gate_home}/.npmrc" ZIZMOR_OFFLINE=true \
    bash "${self}" --as-gate-user "${common}" "${sha}" "$@" {lock}>&-
}

# check_steps prints the steps of `task check`, one per line, but vuln when
# vuln.go.dev cannot be reached. It stops on a step that is no task of its
# own, which could not be run alone, and with task's own status when task
# cannot list them.
check_steps() {
  local line commands vuln_reachable=yes
  if ! curl -fsS -o /dev/null --max-time 10 https://vuln.go.dev/index/db.json.gz; then
    vuln_reachable=""
    # Trade-off TRADE-54: with vuln.go.dev refused, govulncheck cannot read its
    # database, so vuln is left to CI and check's other steps run.
    echo "gate-as-dev: vuln.go.dev cannot be reached from here, so this runs all of check but vuln; CI runs vuln" >&2
  fi
  # A command substitution runs without set -e, so its failure is passed on
  # here rather than read as a check with no steps.
  commands="$(task --summary check | sed -n '/^commands:$/,$p' | sed -n 's/^ - //p')" || exit "$?"
  while IFS= read -r line; do
    if [[ "${line}" != "Task: "* ]]; then
      echo "gate-as-dev: check runs '${line}', which is no task of its own, so it cannot be run a step at a time" >&2
      exit 2
    fi
    [[ "${line}" == "Task: vuln" && -z "${vuln_reachable}" ]] || echo "${line#Task: }"
  done <<<"${commands}"
}

# step_room clears the gate user's own Go build cache when the disk is short
# between steps, and stops when that still leaves too little.
#   step_room <step just run>
step_room() {
  local free
  free="$(free_gb)"
  ((free >= step_need_gb)) && return 0
  go clean -cache
  free="$(free_gb)"
  if ((free < step_floor_gb)); then
    echo "gate-as-dev: only ${free} GB free after $1; stopping before the next step" >&2
    exit 1
  fi
}

# fresh_tmpdir gives the run a temporary directory of the gate user's own.
# Root's tools keep fixed names in /tmp, golangci-lint's lock and the cli test
# worlds among them, and a root-owned 0600 one there refuses the gate user.
# One run holds the lock at a time, so what is in it is a past run's. A test
# binary cut short, by its timeout or by the gate being killed, never runs the
# cleanup that opens a directory it sealed, so everything is opened first.
fresh_tmpdir() {
  export TMPDIR="${HOME}/.cache/tmp"
  if [[ -e "${TMPDIR}" ]]; then chmod -R u+rwx "${TMPDIR}"; fi
  rm -rf "${TMPDIR}"
  mkdir -p "${TMPDIR%/*}"
  mkdir -m 0700 "${TMPDIR}"
}

# run_tasks checks a commit out in the gate user's own checkout and runs the
# tasks asked for there, or the steps of `task check`, one at a time.
#   run_tasks <common git dir> <sha> [task...]
run_tasks() {
  local common="$1" sha="$2" checkout="${HOME}/gate" index status listed
  local -a steps
  shift 2
  if ! command -v task >/dev/null; then
    echo "gate-as-dev: $(id -un) finds no task on PATH; run scripts/cloud-session-setup.sh" >&2
    exit 2
  fi
  fresh_tmpdir
  git config --global --get-all safe.directory | grep -qxF "${common}" \
    || git config --global --add safe.directory "${common}"
  if [[ ! -d "${checkout}/.git" ]]; then
    git clone --quiet --no-checkout "${common}" "${checkout}"
  fi
  git -C "${checkout}" fetch --quiet --no-tags "${common}" "+refs/gate/head:refs/gate/head"
  git -C "${checkout}" checkout --quiet --force --detach "${sha}"
  git -C "${checkout}" clean -ffdxq -e web/node_modules -e tmp/
  cd "${checkout}"
  echo "gate-as-dev: $(git log --oneline -1) as $(id -un)"
  if (($# > 0)); then
    steps=("$@")
  else
    listed="$(check_steps)"
    mapfile -t steps <<<"${listed}"
  fi
  for index in "${!steps[@]}"; do
    status=0
    task "${steps[index]}" || status=$?
    ((status == 0)) || exit "${status}"
    ((index == ${#steps[@]} - 1)) || step_room "${steps[index]}"
  done
}

if [[ "${1:-}" == --as-gate-user ]]; then
  shift
  run_tasks "$@"
  exit 0
fi

if (($# < 1)); then
  echo "Usage: gate-as-dev <worktree> [task...]" >&2
  exit 2
fi

hand_over "$@"
