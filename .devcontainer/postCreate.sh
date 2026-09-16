#!/usr/bin/env bash
# Devcontainer provisioning: install mise, let it install the pinned toolchain
# from mise.toml, then wire the git hooks. Runs once, after the container is
# created — in VS Code, GoLand/Gateway, Codespaces and the devcontainer CLI
# alike, so it assumes nothing about which one started it.
set -euo pipefail

workspace="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly workspace

log() {
  printf '\033[1;34m[ postCreate ]\033[0m %s\n' "$*"
}

export PATH="${HOME}/.local/bin:${PATH}"

if ! command -v mise >/dev/null 2>&1; then
  log "Installing mise (https://mise.run)…"
  curl -fsSL https://mise.run | sh
fi

log "mise trust && mise install — the toolchain pinned in mise.toml"
mise trust "${workspace}/mise.toml"
cd "${workspace}"
mise install

# Activate mise for interactive shells, so `task`, `go` and the linters are on
# PATH without a manual step.
if ! grep -q 'mise activate bash' "${HOME}/.bashrc" 2>/dev/null; then
  cat >>"${HOME}/.bashrc" <<'BASHRC'

# mise (added by .devcontainer/postCreate.sh) — pinned project toolchain
eval "$(mise activate bash)"
BASHRC
fi

log "go mod download"
mise exec -- go mod download

if [[ -d "${workspace}/.git" ]]; then
  log "lefthook install — pre-commit, commit-msg and pre-push hooks"
  mise exec -- lefthook install
else
  log "No .git directory yet; run 'task setup' after 'git init' to add the hooks."
fi

log "Installed toolchain:"
mise ls --current

cat <<'NEXT'

Ready. Useful commands:

  task --list     every task, with a description
  task run        run the TUI from source
  task check      the full gate: lint, tests + coverage floor, vuln, secrets

NEXT
