#!/usr/bin/env bash
#
# Install the tools `task check` and the hooks run, at the versions mise.toml
# and the linters' pin files under build/ pin, into a Claude Code cloud
# session, link them and the gate onto PATH, make the non-root user the gate
# runs as, and install the git hooks.
#
# Usage:
#   scripts/cloud-session-setup.sh
#
# .claude/hooks/session-start.sh runs this when a cloud session starts. The
# session's image carries other versions of Go, golangci-lint and node than
# mise.toml pins, and mise cannot install the pins there: the session refuses
# go.dev, where mise fetches Go, and answers the GitHub API only for the
# repositories in the session, while mise's aqua backend asks it about each
# tool's own repository. GitHub release downloads, nodejs.org,
# proxy.golang.org and sum.golang.org are reachable, so this does what
# build/Dockerfile does: each binary comes from where the Dockerfile fetches it
# and must match its SHA-256 in build/tool-checksums.txt, each Go tool is built
# by `go install` with the pinned Go, which sum.golang.org verifies, and Go
# itself comes from the golang.org/toolchain module. The Go tools build in a
# build cache of their own, removed after; the modules they and Go come from
# stay in the session's module cache. deadcode comes too, for the advisory
# `task deadcode`; hugo-extended, air and cloc, which no gate runs, and vhs
# and ttyd, which `task docs:media` (scripts/docs-media.sh) provides itself,
# are left out.
#
# yamllint and markdownlint-cli2 are pinned outside mise.toml, each one's
# whole tree by hash (Trade-off TRADE-53), and are installed from those files
# as `task tools` installs them: yamllint into a venv of its own from
# build/yamllint/requirements.txt, and markdownlint-cli2 by the pinned node's
# npm ci from a copy of build/markdownlint/package-lock.json, running no
# package's install script.
#
# Each tool's stamp names its pin, for those two the SHA-256 of the pin file,
# and for a Go tool the Go that built it and for markdownlint-cli2 the node it
# runs under, so a bump of either rebuilds what depends on it; a tool whose
# stamp is in place is left alone, and a run with nothing to install takes a
# moment. The links go into ~/.local/bin, which is on the session's PATH
# already: the shell the session runs commands in does not read ~/.bashrc.
# Every run copies scripts/gate-as-dev.sh into the prefix beside the tools, so
# the gate on PATH is root's own and outlives the checkout that ran the setup.
# One setup runs at a time.
#
# The environment is the seam: CLOUD_SETUP_PREFIX (/opt/workflow-tools) is
# where the tools go, CLOUD_SETUP_BIN (~/.local/bin) where they are linked,
# CLOUD_SETUP_GATE_USER (dev) the user it makes for the gate, which
# scripts/gate-as-dev.sh runs as when its GATE_USER names the same user (both
# default to dev), and CLOUD_SETUP_LOCK_WAIT (900) how many seconds to wait
# for another setup.
#
# Exits 0 once the toolchain is in; 1 when an install fails, the disk has no
# room, another setup holds the lock, or the gate user or the git hooks cannot
# be set up; and 2 when mise.toml does not pin a tool this installs, a pin file
# under build/ is missing, or the machine is not x86_64.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo
readonly prefix="${CLOUD_SETUP_PREFIX:-/opt/workflow-tools}"
readonly link_dir="${CLOUD_SETUP_BIN:-${HOME}/.local/bin}"
readonly gate_user="${CLOUD_SETUP_GATE_USER:-dev}"
readonly lock_wait="${CLOUD_SETUP_LOCK_WAIT:-900}"
readonly checksums="${repo}/build/tool-checksums.txt"
readonly requirements="${repo}/build/yamllint/requirements.txt"
readonly npm_pins="${repo}/build/markdownlint"
readonly arch=amd64
# The room an install from nothing takes: the tools, their downloads, and the
# Go tools' build cache while they build.
readonly need_gb=3

# pin prints the version mise.toml pins for a [tools] key, or stops the setup.
#   pin <key>
pin() {
  local key="$1" escaped value
  escaped="${key//\//\\/}"
  value="$(sed -n -E "s/^\"?${escaped}\"?[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\1/p" "${repo}/mise.toml" | head -1)"
  if [[ -z "${value}" ]]; then
    echo "cloud-session-setup: no pin for '${key}' in mise.toml" >&2
    exit 2
  fi
  printf '%s' "${value}"
}

# pin_digest prints the SHA-256 of a pin file under build/, or stops the setup.
# sha256sum is GNU's; stock macOS has only shasum.
#   pin_digest <file>
pin_digest() {
  local digest
  if [[ ! -f "$1" ]]; then
    echo "cloud-session-setup: no pin file ${1#"${repo}/"}" >&2
    exit 2
  fi
  if command -v sha256sum >/dev/null; then
    digest="$(sha256sum "$1")"
  else
    digest="$(shasum -a 256 "$1")"
  fi
  printf '%s' "${digest%% *}"
}

machine="$(uname -m)"
if [[ "${machine}" != x86_64 ]]; then
  echo "cloud-session-setup: this setup fetches only the x86_64 builds, not ${machine}" >&2
  exit 2
fi

go_version="$(pin go)"
node_version="$(pin node)"
shellcheck_version="$(pin shellcheck)"
jq_version="$(pin jq)"
lefthook_version="$(pin lefthook)"
typos_version="$(pin typos)"
hadolint_version="$(pin hadolint)"
taplo_version="$(pin taplo)"
zizmor_version="$(pin zizmor)"
task_version="$(pin task)"
golangci_lint_version="$(pin golangci-lint)"
shfmt_version="$(pin shfmt)"
actionlint_version="$(pin actionlint)"
govulncheck_version="$(pin 'go:golang.org/x/vuln/cmd/govulncheck')"
gitleaks_version="$(pin 'go:github.com/zricethezav/gitleaks/v8')"
gobco_version="$(pin 'go:github.com/rillig/gobco')"
deadcode_version="$(pin 'go:golang.org/x/tools/cmd/deadcode')"
yamllint_pins="$(pin_digest "${requirements}")"
# npm ci refuses a package.json its lock does not match, so the lock changes
# whenever what package.json asks for does, and its digest names the stamp
# alone. package.json is checked here all the same, outside a command
# substitution, so that one missing stops the setup with 2 too.
markdownlint_pins="$(pin_digest "${npm_pins}/package-lock.json")"
pin_digest "${npm_pins}/package.json" >/dev/null

mkdir -p "${prefix}/bin" "${prefix}/stamps" "${link_dir}"
exec {lock}>"${prefix}/.setup.lock"
if ! flock -w "${lock_wait}" "${lock}"; then
  echo "cloud-session-setup: another setup has held ${prefix}/.setup.lock for ${lock_wait} s" >&2
  exit 1
fi

work="$(mktemp -d)"
readonly work
# The tool being installed, named when its installer stops the setup.
installing=""
# Whether this run has checked the disk has room for an install.
room_checked=""

# finish removes the downloads and, when an installer stopped the setup, says
# which tool it was.
finish() {
  local status=$?
  rm -rf "${work}"
  if ((status != 0)) && [[ -n "${installing}" ]]; then
    echo "cloud-session-setup: ${installing} was not installed" >&2
    exit 1
  fi
}
trap finish EXIT

# check_room stops the setup before its first install when the disk has no
# room for one. The session's disk is a fixed allowance that df reports as
# free space.
check_room() {
  local free
  [[ -n "${room_checked}" ]] && return 0
  free="$(df -Pk / | awk 'NR == 2 { print int($4 / 1048576) }')"
  if ((free < need_gb)); then
    echo "cloud-session-setup: an install needs ${need_gb} GB free on /, and ${free} GB is" >&2
    exit 1
  fi
  room_checked=yes
}

# ensure runs an installer unless the tool's stamp for this version is in
# place. Before installing it drops the tool's other stamps, and it writes
# this one only once the installer has succeeded, so a failed install is
# retried on the next run. The installer runs as a plain command, so set -e
# stops the setup at the first step that fails.
#   ensure <tool> <stamp version> <installer> [arg...]
ensure() {
  local tool="$1" stamp="${prefix}/stamps/$1@$2"
  [[ -e "${stamp}" ]] && return 0
  check_room
  installing="$1 $2"
  shift 2
  rm -f "${prefix}/stamps/${tool}@"*
  "$@"
  : >"${stamp}"
  installing=""
}

# fetch_verified downloads a file and stops unless it matches its pinned hash.
#   fetch_verified <url> <file> <tool> <version>
fetch_verified() {
  curl -fsSL "$1" -o "${work}/$2"
  bash "${repo}/scripts/verify-download.sh" "${checksums}" "${work}/$2" "$3" "$4" "${arch}"
}

# swap_in moves a directory built under the prefix, beside the tool it
# replaces, into that tool's place, so the tool there now stays whole until
# the new one is ready.
#   swap_in <built dir> <name under the prefix>
swap_in() {
  rm -rf "${prefix}/$2.old"
  if [[ -e "${prefix}/$2" ]]; then mv "${prefix}/$2" "${prefix}/$2.old"; fi
  mv "$1" "${prefix}/$2"
  rm -rf "${prefix}/$2.old"
}

install_go() {
  local root
  root="$(GOTOOLCHAIN="go${go_version}" go env GOROOT)"
  # A run cut short leaves go.new behind, and cp -a would copy into it.
  rm -rf "${prefix}/go.new"
  cp -a "${root}" "${prefix}/go.new"
  chmod -R u+w "${prefix}/go.new"
  swap_in "${prefix}/go.new" go
}

# Run as root, tar keeps the owner each archive names, a uid of whoever built
# the release, so every tar below passes --no-same-owner: what it unpacks is
# the unpacking user's.
install_node() {
  fetch_verified "https://nodejs.org/dist/v${node_version}/node-v${node_version}-linux-x64.tar.xz" \
    node.tar.xz node "${node_version}"
  rm -rf "${prefix}/node.new"
  mkdir -p "${prefix}/node.new"
  tar -xJf "${work}/node.tar.xz" -C "${prefix}/node.new" --strip-components=1 --no-same-owner
  swap_in "${prefix}/node.new" node
}

install_shellcheck() {
  fetch_verified "https://github.com/koalaman/shellcheck/releases/download/v${shellcheck_version}/shellcheck-v${shellcheck_version}.linux.x86_64.tar.xz" \
    shellcheck.tar.xz shellcheck "${shellcheck_version}"
  tar -xJf "${work}/shellcheck.tar.xz" -C "${prefix}/bin" --strip-components=1 --no-same-owner \
    "shellcheck-v${shellcheck_version}/shellcheck"
}

install_jq() {
  fetch_verified "https://github.com/jqlang/jq/releases/download/jq-${jq_version}/jq-linux-amd64" \
    jq jq "${jq_version}"
  install -m 0755 "${work}/jq" "${prefix}/bin/jq"
}

# lefthook and taplo come gzipped, so each is unpacked beside the downloads and
# then installed, which replaces the tool rather than writing into it: a write
# into one another lane's hook is running fails with "Text file busy".
install_lefthook() {
  fetch_verified "https://github.com/evilmartians/lefthook/releases/download/v${lefthook_version}/lefthook_${lefthook_version}_Linux_x86_64.gz" \
    lefthook.gz lefthook "${lefthook_version}"
  gunzip -c "${work}/lefthook.gz" >"${work}/lefthook"
  install -m 0755 "${work}/lefthook" "${prefix}/bin/lefthook"
}

install_typos() {
  fetch_verified "https://github.com/crate-ci/typos/releases/download/v${typos_version}/typos-v${typos_version}-x86_64-unknown-linux-musl.tar.gz" \
    typos.tar.gz typos "${typos_version}"
  tar -xzf "${work}/typos.tar.gz" -C "${prefix}/bin" --strip-components=1 --no-same-owner ./typos
}

install_hadolint() {
  fetch_verified "https://github.com/hadolint/hadolint/releases/download/v${hadolint_version}/hadolint-Linux-x86_64" \
    hadolint hadolint "${hadolint_version}"
  install -m 0755 "${work}/hadolint" "${prefix}/bin/hadolint"
}

install_taplo() {
  fetch_verified "https://github.com/tamasfe/taplo/releases/download/${taplo_version}/taplo-linux-x86_64.gz" \
    taplo.gz taplo "${taplo_version}"
  gunzip -c "${work}/taplo.gz" >"${work}/taplo"
  install -m 0755 "${work}/taplo" "${prefix}/bin/taplo"
}

install_zizmor() {
  fetch_verified "https://github.com/zizmorcore/zizmor/releases/download/v${zizmor_version}/zizmor-x86_64-unknown-linux-gnu.tar.gz" \
    zizmor.tar.gz zizmor "${zizmor_version}"
  tar -xzf "${work}/zizmor.tar.gz" -C "${prefix}/bin" --no-same-owner zizmor
}

# go_install builds a Go tool with the pinned Go into the prefix, in a build
# cache of its own.
#   go_install <module path>@<version>
go_install() {
  GOBIN="${prefix}/bin" GOCACHE="${work}/gocache" GOTOOLCHAIN=local GOFLAGS="" \
    "${prefix}/go/bin/go" install "$1"
}

# install_yamllint installs yamllint and its tree into a venv of its own, from
# the requirements file that pins every package by each wheel's sha256.
install_yamllint() {
  rm -rf "${prefix}/venv"
  python3 -m venv "${prefix}/venv"
  "${prefix}/venv/bin/pip" install --quiet --no-cache-dir --disable-pip-version-check -r "${requirements}"
  ln -sfn "${prefix}/venv/bin/yamllint" "${prefix}/bin/yamllint"
}

# install_markdownlint installs markdownlint-cli2 with the pinned node's own
# npm, from a copy of the lock that pins every package by its sha512: npm ci
# takes exactly what the lock names, and --ignore-scripts runs no package's
# install script. It is built beside the one in place and then swapped in.
install_markdownlint() {
  rm -rf "${prefix}/markdownlint.new"
  mkdir -p "${prefix}/markdownlint.new"
  cp "${npm_pins}/package.json" "${npm_pins}/package-lock.json" "${prefix}/markdownlint.new/"
  PATH="${prefix}/node/bin:${PATH}" npm ci --prefix "${prefix}/markdownlint.new" --ignore-scripts \
    --no-fund --no-audit
  swap_in "${prefix}/markdownlint.new" markdownlint
  ln -sfn "${prefix}/markdownlint/node_modules/.bin/markdownlint-cli2" "${prefix}/bin/markdownlint-cli2"
}

# link_tools links every installed tool, the gate's copy among them, into the
# link directory. The prefix's own bin comes last, so a tool installed there
# wins over one of the same name a runtime's bin holds, as an older setup's
# markdownlint-cli2 in node's.
link_tools() {
  local tool
  for tool in "${prefix}"/go/bin/* "${prefix}"/node/bin/* "${prefix}"/bin/*; do
    if [[ -e "${tool}" ]]; then
      ln -sfn "${tool}" "${link_dir}/$(basename "${tool}")"
    fi
  done
}

readonly built_by="go${go_version}"
ensure go "${go_version}" install_go
ensure node "${node_version}" install_node
ensure shellcheck "${shellcheck_version}" install_shellcheck
ensure jq "${jq_version}" install_jq
ensure lefthook "${lefthook_version}" install_lefthook
ensure typos "${typos_version}" install_typos
ensure hadolint "${hadolint_version}" install_hadolint
ensure taplo "${taplo_version}" install_taplo
ensure zizmor "${zizmor_version}" install_zizmor
ensure task "${task_version}+${built_by}" go_install "github.com/go-task/task/v3/cmd/task@v${task_version}"
ensure golangci-lint "${golangci_lint_version}+${built_by}" \
  go_install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${golangci_lint_version}"
ensure shfmt "${shfmt_version}+${built_by}" go_install "mvdan.cc/sh/v3/cmd/shfmt@v${shfmt_version}"
ensure actionlint "${actionlint_version}+${built_by}" \
  go_install "github.com/rhysd/actionlint/cmd/actionlint@v${actionlint_version}"
ensure govulncheck "${govulncheck_version}+${built_by}" \
  go_install "golang.org/x/vuln/cmd/govulncheck@v${govulncheck_version}"
ensure gitleaks "${gitleaks_version}+${built_by}" go_install "github.com/zricethezav/gitleaks/v8@v${gitleaks_version}"
ensure gobco "${gobco_version}+${built_by}" go_install "github.com/rillig/gobco@v${gobco_version}"
ensure deadcode "${deadcode_version}+${built_by}" go_install "golang.org/x/tools/cmd/deadcode@v${deadcode_version}"
ensure yamllint "${yamllint_pins}" install_yamllint
ensure markdownlint-cli2 "${markdownlint_pins}+node${node_version}" install_markdownlint
install -m 0755 "${repo}/scripts/gate-as-dev.sh" "${prefix}/bin/gate-as-dev"
chmod -R a+rX "${prefix}"
link_tools

if ! id "${gate_user}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${gate_user}" || {
    echo "cloud-session-setup: could not make the gate user ${gate_user}" >&2
    exit 1
  }
fi

if git -C "${repo}" rev-parse --git-dir >/dev/null 2>&1; then
  (cd "${repo}" && lefthook install) || {
    echo "cloud-session-setup: lefthook could not install the git hooks" >&2
    exit 1
  }
fi

echo "cloud-session-setup: toolchain ready in ${prefix}, linked into ${link_dir}"
