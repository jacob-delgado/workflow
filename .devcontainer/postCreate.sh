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

# mise itself is pinned, and verified against checksums written HERE rather
# than fetched beside the download: a checksum file from the same release page
# proves the download was not truncated, not that it was not replaced. The
# version follows the same week-long age gate as every pin in mise.toml.
readonly mise_version="2026.9.3"

# install_mise downloads the pinned release for this architecture and installs
# it only if it matches the SHA-256 published in that release's SHASUMS256.txt.
install_mise() {
  local arch expected tarball download
  case "$(uname -m)" in
    x86_64 | amd64)
      arch="x64"
      expected="72de46e58238e3ae860e449adb9a35f4aba53679621c214dec03d68955fa4de8"
      ;;
    aarch64 | arm64)
      arch="arm64"
      expected="d1918155164a7e4ae4ae306e259b186065c0cfc1b993a954d6e033af0fa4fc5a"
      ;;
    *)
      echo "no pinned mise for $(uname -m)" >&2
      return 1
      ;;
  esac

  tarball="mise-v${mise_version}-linux-${arch}.tar.gz"
  download="$(mktemp -d)"

  # Every step returns explicitly. set -e is switched off inside a function
  # called from an `if` or `||`, so relying on it here would let a download
  # that fails its checksum be installed anyway — which a test of this exact
  # function did.
  if ! curl -fsSL -o "${download}/${tarball}" \
    "https://github.com/jdx/mise/releases/download/v${mise_version}/${tarball}"; then
    rm -rf "${download}"
    return 1
  fi

  if ! echo "${expected}  ${download}/${tarball}" | sha256sum --check --strict -; then
    echo "mise ${mise_version} does not match its pinned checksum; not installing it" >&2
    rm -rf "${download}"
    return 1
  fi

  mkdir -p "${HOME}/.local/bin"
  if ! tar -xzf "${download}/${tarball}" -C "${download}" \
    || ! install -m 0755 "${download}/mise/bin/mise" "${HOME}/.local/bin/mise"; then
    rm -rf "${download}"
    return 1
  fi

  rm -rf "${download}"
}

if ! command -v mise >/dev/null 2>&1; then
  log "Installing mise ${mise_version}, checksum-verified…"
  install_mise
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
  task check      the full gate: Go and web lint + tests, coverage, vuln, secrets

NEXT
