# shellcheck shell=bash
# shellcheck disable=SC2154 # here and workdir are the sourcing test's
#
# The fixtures cloud-session-setup.sh's two tests build their checkouts from:
# stand-ins for curl, go, lefthook, useradd, id, uname and df, which log what
# they were asked to the call log; the mise.toml pins every case starts from,
# and the stamps a finished setup holds at them; a checkout laid out with the
# setup script, a prefix filled as a finished setup leaves it, a checksum
# pinned for a served file, and the run of the setup with the stand-ins, a
# prefix and a link directory of its own. cloud-session-setup_test.sh drives
# what the setup fetches, builds, links and stops on;
# cloud-session-setup_pins_test.sh what it reads its pins from.
#
# Sourced, never run, after lib/testing.sh, which sets ${workdir}; the setup
# a checkout copies is the one beside the sourcing test's ${here}:
#
#   # shellcheck source=lib/cloud-session-setup-fixtures.sh
#   source "${here}/lib/cloud-session-setup-fixtures.sh"

readonly stub_bin="${workdir}/stubs"
readonly log="${workdir}/calls.log"
readonly served="${workdir}/served"
mkdir -p "${stub_bin}" "${served}"

# stub writes a stand-in command that logs its name and arguments.
#   stub <name> [body]
stub() {
  printf '#!/usr/bin/env bash\nprintf "%%s %%s\\n" "%s" "$*" >>"%s"\n%s\n' \
    "$1" "${log}" "${2:-}" >"${stub_bin}/$1"
  chmod +x "${stub_bin}/$1"
}

# curl serves a file from ${served} named as the URL's last part, and fails
# as curl -f does for anything else. It writes a new file, as curl -o does,
# so a download never keeps the served file's mode.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub curl 'out="" url=""
while (($#)); do case "$1" in -o) out="$2"; shift ;; http*) url="$1" ;; esac; shift; done
file="'"${served}"'/${url##*/}"
[[ -f "${file}" ]] || exit 22
cat "${file}" >"${out}"'
# go reports GO_ROOT as the root of the Go it is asked for, and logs which
# toolchain that was.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub go 'if [[ "$*" == "env GOROOT" ]]; then
  printf "toolchain %s\n" "${GOTOOLCHAIN:-}" >>"'"${log}"'"
  printf "%s\n" "${GO_ROOT}"
fi'
# lefthook fails outside a repository, as the real one does.
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub lefthook 'git rev-parse --git-dir >/dev/null 2>&1 || exit 128
exit "${LEFTHOOK_STATUS:-0}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub useradd 'exit "${USERADD_STATUS:-0}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub id 'exit "${ID_STATUS:-0}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub uname 'printf "%s\n" "${UNAME_M:-x86_64}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub df 'printf "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/vda 1 1 %d 1%% /\n" $((${DF_FREE_GB:-50} * 1048576))'

# The pins every case starts from; a case moves or removes one.
readonly pins='go = "1.27.2"
task = "3.54.0"
golangci-lint = "2.14.0"
lefthook = "2.2.1"
shellcheck = "0.11.0"
shfmt = "3.14.1"
typos = "1.51.1"
actionlint = "1.7.12"
hadolint = "2.15.1"
taplo = "0.10.0"
zizmor = "1.30.1"
rumdl = "0.3.3"
node = "24.21.0"
jq = "1.8.2"
"go:golang.org/x/vuln/cmd/govulncheck" = "1.9.0"
"go:github.com/rillig/gobco" = "1.3.4"
"go:github.com/zricethezav/gitleaks/v8" = "8.30.1"
"go:golang.org/x/tools/cmd/deadcode" = "0.52.0"'

# fixture_pin_file prints the mise.toml of the checkout every case starts
# from, the one file the setup reads its pins from.
#   fixture_pin_file <path>
fixture_pin_file() {
  case "$1" in
    mise.toml) printf '[tools]\n%s\n' "${pins}" ;;
    *) return 1 ;;
  esac
}

# digest_of prints a file's SHA-256, with GNU's sha256sum or, as on stock
# macOS, shasum.
#   digest_of <file>
digest_of() {
  local digest
  if command -v sha256sum >/dev/null; then
    digest="$(sha256sum "$1")"
  else
    digest="$(shasum -a 256 "$1")"
  fi
  printf '%s' "${digest%% *}"
}

# shellcheck source=toolchain-pins.sh
source "${here}/lib/toolchain-pins.sh"

# The toolchain's own stamp at those pins, the digest of every pin the setup
# installs from as the library the gate reads a commit with reads them: the
# tests hold the setup to it, so the gate and the setup read pins alike.
pins_digest="$(toolchain_digest fixture_pin_file)"
readonly pins_digest

# Every stamp a finished setup holds at those pins. A Go tool's names the Go it
# was built with.
readonly stamps="pins@${pins_digest} go@1.27.2 node@24.21.0 shellcheck@0.11.0 jq@1.8.2 lefthook@2.2.1
typos@1.51.1 hadolint@2.15.1 taplo@0.10.0 zizmor@1.30.1 rumdl@0.3.3
task@3.54.0+go1.27.2 golangci-lint@2.14.0+go1.27.2 shfmt@3.14.1+go1.27.2
actionlint@1.7.12+go1.27.2 govulncheck@1.9.0+go1.27.2 gitleaks@8.30.1+go1.27.2
gobco@1.3.4+go1.27.2 deadcode@0.52.0+go1.27.2"

# repo lays out a checkout with the setup script, the download check it calls,
# the library it reads pins with, the gate it installs, a mise.toml and an
# empty checksums file.
#   repo <dir>
repo() {
  mkdir -p "$1/scripts/lib" "$1/build"
  cp "${here}/cloud-session-setup.sh" "${here}/verify-download.sh" "$1/scripts/"
  cp "${here}/lib/toolchain-pins.sh" "$1/scripts/lib/"
  printf '#!/usr/bin/env bash\n# the gate\n' >"$1/scripts/gate-as-dev.sh"
  printf '[tools]\n%s\n' "${pins}" >"$1/mise.toml"
  : >"$1/build/tool-checksums.txt"
}

# repin moves or removes a checkout's pins with a sed script, in the form both
# GNU's and BSD's sed take.
#   repin <dir> <sed script>
repin() {
  sed -i.bak "$2" "$1/mise.toml" && rm "$1/mise.toml.bak"
}

# installed fills a prefix as a finished setup leaves it: every stamp, and a
# tool in each directory the setup links from, made unreadable to others.
#   installed <prefix>
installed() {
  local stamp
  mkdir -p "$1/stamps" "$1/bin" "$1/go/bin" "$1/node/bin"
  for stamp in ${stamps}; do : >"$1/stamps/${stamp}"; done
  install -m 0700 /dev/null "$1/bin/task"
  install -m 0700 /dev/null "$1/go/bin/go"
  install -m 0700 /dev/null "$1/node/bin/node"
}

# pin_checksum pins a served file's SHA-256 for a tool in a checkout.
#   pin_checksum <dir> <tool> <version> <file>
pin_checksum() {
  printf '%s %s amd64 %s\n' "$2" "$3" "$(digest_of "$4")" >>"$1/build/tool-checksums.txt"
}

# setup runs a checkout's setup script with the stand-ins, a prefix, a link
# directory and the gate's two locks of its own, and a fresh call log.
# GOTOOLCHAIN, GOBIN, GOFLAGS and GOCACHE are set to what a Go tool must not
# be built with, so a build that leaves one to the environment shows.
#   setup <dir> [VAR=value...]
setup() {
  local dir="$1"
  shift
  : >"${log}"
  env PATH="${stub_bin}:${PATH}" HOME="${dir}.home" GO_ROOT="${dir}.goroot" \
    GOTOOLCHAIN=auto GOBIN="${dir}.gobin" GOFLAGS=-mod=vendor GOCACHE="${dir}.gocache" \
    CLOUD_SETUP_PREFIX="${dir}.prefix" CLOUD_SETUP_BIN="${dir}.bin" \
    GATE_LOCK="${dir}.gate.lock" GATE_RUN_LOCK="${dir}.gate-run.lock" "$@" \
    "${dir}/scripts/cloud-session-setup.sh"
}
