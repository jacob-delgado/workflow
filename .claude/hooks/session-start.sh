#!/usr/bin/env bash
#
# SessionStart hook: in a Claude Code cloud session, install the toolchain
# mise.toml pins before the session's first command, with
# scripts/cloud-session-setup.sh. Outside a cloud session it does nothing.
#
# What a SessionStart hook prints becomes context for the session, so the
# setup's own output goes to a log (CLOUD_SETUP_LOG) and the hook says in one
# line whether the toolchain is ready and where the log is. The log is in
# ~/.cache, where no other user can leave a link or a pipe for root to open,
# as one could at a fixed name in /tmp. A failed setup does not stop the
# session from starting: the line says so, and what to do. The setup gets
# CLOUD_SETUP_TIMEOUT seconds, under the hook's own timeout in
# .claude/settings.json, so the line still reaches the session when it runs
# out of time.
#
# CLAUDE_ENV_FILE holds what every command the session runs starts with, so
# the hook adds to it, once each: the link directory on PATH, whatever the
# shell's own PATH says; zizmor offline (TRADE-54), for the hooks and
# `task lint` run as root; and corepack pointed at the npm registry, since a
# cloud session cannot resolve repo.yarnpkg.com, where corepack fetches Yarn
# otherwise.
set -euo pipefail

if [[ "${CLAUDE_CODE_REMOTE:-}" != "true" ]]; then
  exit 0
fi

project="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
readonly project
readonly log="${CLOUD_SETUP_LOG:-${HOME}/.cache/cloud-session-setup.log}"
readonly time_limit="${CLOUD_SETUP_TIMEOUT:-1080}"

# export_once adds a line to the session's environment file unless it holds
# it already.
#   export_once <line>
export_once() {
  grep -qxF "$1" "${CLAUDE_ENV_FILE}" 2>/dev/null || printf '%s\n' "$1" >>"${CLAUDE_ENV_FILE}"
}

if [[ -n "${CLAUDE_ENV_FILE:-}" ]]; then
  # shellcheck disable=SC2016 # expanded when each command starts, not here
  export_once 'export PATH="${HOME}/.local/bin:${PATH}"'
  # Trade-off TRADE-54: zizmor runs offline for the hooks and task lint run as
  # root: its online audits read third-party actions' tags from github.com,
  # which refuses the session's GitHub token. CI's zizmor has no token and
  # runs offline too.
  export_once 'export ZIZMOR_OFFLINE=true'
  # The session cannot resolve repo.yarnpkg.com. Over the npm registry corepack
  # checks no signature on Yarn, only the SHA-512 web/package.json pins.
  export_once 'export COREPACK_NPM_REGISTRY=https://registry.npmjs.org'
fi

status=0
mkdir -p "$(dirname "${log}")"
timeout "${time_limit}" "${project}/scripts/cloud-session-setup.sh" >"${log}" 2>&1 || status=$?
if ((status == 0)); then
  echo "cloud-session-setup: the toolchain mise.toml pins is ready; run the gate as the dev user with gate-as-dev <worktree> (log: ${log})"
elif ((status == 124)); then
  echo "cloud-session-setup ran out of its ${time_limit} s: the session may have its image's tools, not mise.toml's pins. Read ${log}, then rerun scripts/cloud-session-setup.sh."
else
  echo "cloud-session-setup failed (exit ${status}): the session may have its image's tools, not mise.toml's pins. Read ${log}, then rerun scripts/cloud-session-setup.sh."
fi
