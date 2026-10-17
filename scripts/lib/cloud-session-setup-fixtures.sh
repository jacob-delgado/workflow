# shellcheck shell=bash
# shellcheck disable=SC2154 # here and workdir are the sourcing test's
#
# The fixtures cloud-session-setup.sh's two tests build their checkouts from:
# stand-ins for curl, go, python3, pip, npm, lefthook, useradd, id, uname and
# df, which log what they were asked to the call log; the mise.toml pins and
# the linters' pin files every case starts from, and the stamps a finished
# setup holds at them; a checkout laid out with the setup script, a prefix
# filled as a finished setup leaves it, a checksum pinned for a served file,
# and the run of the setup with the stand-ins, a prefix and a link directory
# of its own. cloud-session-setup_test.sh drives what the setup fetches,
# builds, links and stops on; cloud-session-setup_pins_test.sh what it reads
# its pins from, and how it installs the two linters pinned under build/.
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
# python3 -m venv makes a venv whose pip is a stand-in too.
stub pip
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub python3 'if [[ "$1 $2" == "-m venv" ]]; then
  mkdir -p "$3/bin" && cp "'"${stub_bin}"'/pip" "$3/bin/pip" && : >"$3/bin/yamllint"
fi'
# npm ci puts a markdownlint-cli2 where the lock it is given installs one.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub npm 'if [[ "$1" == ci ]]; then
  while (($#)); do if [[ "$1" == --prefix ]]; then dir="$2"; fi; shift; done
  mkdir -p "${dir}/node_modules/.bin" && : >"${dir}/node_modules/.bin/markdownlint-cli2"
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
node = "24.21.0"
jq = "1.8.2"
"go:golang.org/x/vuln/cmd/govulncheck" = "1.9.0"
"go:github.com/rillig/gobco" = "1.3.4"
"go:github.com/zricethezav/gitleaks/v8" = "8.30.1"
"go:golang.org/x/tools/cmd/deadcode" = "0.52.0"'

# The two linters mise.toml does not pin, each pinned by a file under build/
# whose SHA-256 names its stamp.
readonly requirements='--require-hashes
yamllint==1.38.0 --hash=sha256:aaaa'
readonly package_json='{"dependencies": {"markdownlint-cli2": "0.23.3"}}'
readonly package_lock='{"lockfileVersion": 3}'

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

printf '%s\n' "${requirements}" >"${workdir}/requirements.txt"
printf '%s\n' "${package_lock}" >"${workdir}/package-lock.json"
requirements_digest="$(digest_of "${workdir}/requirements.txt")"
lock_digest="$(digest_of "${workdir}/package-lock.json")"
readonly requirements_digest lock_digest

# Every stamp a finished setup holds at those pins. A Go tool's names the Go it
# was built with, yamllint's its requirements file, and markdownlint's its lock
# and the node it was installed under.
readonly stamps="go@1.27.2 node@24.21.0 shellcheck@0.11.0 jq@1.8.2 lefthook@2.2.1
typos@1.51.1 hadolint@2.15.1 taplo@0.10.0 zizmor@1.30.1 yamllint@${requirements_digest}
task@3.54.0+go1.27.2 golangci-lint@2.14.0+go1.27.2 shfmt@3.14.1+go1.27.2
actionlint@1.7.12+go1.27.2 govulncheck@1.9.0+go1.27.2 gitleaks@8.30.1+go1.27.2
gobco@1.3.4+go1.27.2 deadcode@0.52.0+go1.27.2 markdownlint-cli2@${lock_digest}+node24.21.0"

# repo lays out a checkout with the setup script, the download check it calls,
# the gate it installs, a mise.toml, an empty checksums file, and the linters'
# pin files.
#   repo <dir>
repo() {
  mkdir -p "$1/scripts" "$1/build/yamllint" "$1/build/markdownlint"
  cp "${here}/cloud-session-setup.sh" "${here}/verify-download.sh" "$1/scripts/"
  printf '#!/usr/bin/env bash\n# the gate\n' >"$1/scripts/gate-as-dev.sh"
  printf '[tools]\n%s\n' "${pins}" >"$1/mise.toml"
  : >"$1/build/tool-checksums.txt"
  printf '%s\n' "${requirements}" >"$1/build/yamllint/requirements.txt"
  printf '%s\n' "${package_json}" >"$1/build/markdownlint/package.json"
  printf '%s\n' "${package_lock}" >"$1/build/markdownlint/package-lock.json"
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

# setup runs a checkout's setup script with the stand-ins, a prefix and a
# link directory of its own, and a fresh call log. GOTOOLCHAIN, GOBIN,
# GOFLAGS and GOCACHE are set to what a Go tool must not be built with, so a
# build that leaves one to the environment shows.
#   setup <dir> [VAR=value...]
setup() {
  local dir="$1"
  shift
  : >"${log}"
  env PATH="${stub_bin}:${PATH}" HOME="${dir}.home" GO_ROOT="${dir}.goroot" \
    GOTOOLCHAIN=auto GOBIN="${dir}.gobin" GOFLAGS=-mod=vendor GOCACHE="${dir}.gocache" \
    CLOUD_SETUP_PREFIX="${dir}.prefix" CLOUD_SETUP_BIN="${dir}.bin" "$@" \
    "${dir}/scripts/cloud-session-setup.sh"
}
