#!/usr/bin/env bash
#
# Tests for scripts/cloud-session-setup.sh: a finished setup fetches nothing
# again and leaves the tools, the gate's own copy among them, linked and
# readable, and the run after an install fetches nothing either; every download
# lands where the setup links it from, runnable by anyone, and a tool installed
# again while it runs is replaced; a download that fails or misses its pinned
# SHA-256 is not installed, and the setup names the tool and exits 1; a tool
# mise.toml does not pin stops the setup; a Go already in the prefix survives a
# repair, and a Go build a run cut short left behind is not built into; a Go or
# node bump rebuilds what was built with the old one, and leaves a Go copied
# from the module cache's read-only tree writable for the next, each Go tool
# built into the prefix by the pinned Go alone, with none of the environment's
# flags and in a build cache of the setup's own, removed after, and
# markdownlint-cli2 installed by the pinned node's own npm, which takes no
# dependency released within the week; yamllint is installed again at its pin
# when its stamp is gone; run as root, no archive's owner is kept; the setup
# stops before it fetches on a disk without room and on a machine it has no
# downloads for, waits for another setup and stops when that one holds the lock
# too long; and the gate user is made, with a home, only when it is missing, a
# gate user or git hooks that cannot be set up exiting 1 too, without blaming a
# tool the run installed first; and git hooks are installed only in a checkout
# that is a repository. Stand-ins for curl, go, python3, pip, npm, df, uname,
# id, useradd and lefthook record what they were asked, and curl serves only the
# files a case puts in place, by name, so nothing is downloaded, built or
# installed for real.
#
# Usage:
#   scripts/cloud-session-setup_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The setup takes its lock with flock, which util-linux has and stock macOS
# does not; the script is for a Linux cloud session alone.
command -v flock >/dev/null || {
  echo "cloud-session-setup_test.sh: skipped, no flock here"
  exit 0
}

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
stub npm
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
yamllint = "1.38.0"
typos = "1.51.1"
actionlint = "1.7.12"
hadolint = "2.15.1"
taplo = "0.10.0"
zizmor = "1.30.1"
node = "24.21.0"
"npm:markdownlint-cli2" = "0.23.3"
jq = "1.8.2"
"go:golang.org/x/vuln/cmd/govulncheck" = "1.9.0"
"go:github.com/rillig/gobco" = "1.3.4"
"go:github.com/zricethezav/gitleaks/v8" = "8.30.1"
"go:golang.org/x/tools/cmd/deadcode" = "0.52.0"'

# Every stamp a finished setup holds at those pins. A Go tool's names the Go it
# was built with, and markdownlint's the node it was installed under.
readonly stamps='go@1.27.2 node@24.21.0 shellcheck@0.11.0 jq@1.8.2 lefthook@2.2.1
typos@1.51.1 hadolint@2.15.1 taplo@0.10.0 zizmor@1.30.1 yamllint@1.38.0
task@3.54.0+go1.27.2 golangci-lint@2.14.0+go1.27.2 shfmt@3.14.1+go1.27.2
actionlint@1.7.12+go1.27.2 govulncheck@1.9.0+go1.27.2 gitleaks@8.30.1+go1.27.2
gobco@1.3.4+go1.27.2 deadcode@0.52.0+go1.27.2 markdownlint-cli2@0.23.3+node24.21.0'

# repo lays out a checkout with the setup script, the download check it calls,
# the gate it installs, a mise.toml and an empty checksums file.
#   repo <dir>
repo() {
  mkdir -p "$1/scripts" "$1/build"
  cp "${here}/cloud-session-setup.sh" "${here}/verify-download.sh" "$1/scripts/"
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

# pin_checksum pins a served file's SHA-256 for a tool in a checkout, with
# GNU's sha256sum or, as on stock macOS, shasum.
#   pin_checksum <dir> <tool> <version> <file>
pin_checksum() {
  local digest
  if command -v sha256sum >/dev/null; then
    digest="$(sha256sum "$4")"
  else
    digest="$(shasum -a 256 "$4")"
  fi
  printf '%s %s amd64 %s\n' "$2" "$3" "${digest%% *}" >>"$1/build/tool-checksums.txt"
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

done_repo="${workdir}/done"
repo "${done_repo}"
git -C "${done_repo}" init -q
installed "${done_repo}.prefix"
expect_output pass "a finished setup runs again" "toolchain ready" setup "${done_repo}"
count_case
if grep -qE '^(curl|go|python3|npm|df) ' "${log}"; then
  fail_case "a tool already at its pin" "it was fetched or measured again: $(grep -E '^(curl|go|python3|npm|df) ' "${log}" | head -1)"
fi
for tool in bin/task go/bin/go node/bin/node; do
  count_case
  if [[ "$(readlink "${done_repo}.bin/${tool##*/}")" != "${done_repo}.prefix/${tool}" ]]; then
    fail_case "the tools onto PATH" "${tool##*/} is not linked from the prefix's ${tool%/*}"
  fi
done
count_case
if [[ "$(readlink "${done_repo}.bin/gate-as-dev")" != "${done_repo}.prefix/bin/gate-as-dev" ]] \
  || ! cmp -s "${done_repo}/scripts/gate-as-dev.sh" "${done_repo}.prefix/bin/gate-as-dev"; then
  fail_case "the gate onto PATH" "gate-as-dev is not linked to a copy of scripts/gate-as-dev.sh in the prefix"
fi
count_case
if [[ -z "$(find "${done_repo}.prefix/bin/task" -perm -o=rx)" ]]; then
  fail_case "the tools for the gate user" "the prefix is not readable by others: $(ls -l "${done_repo}.prefix/bin/task")"
fi
count_case
if ! grep -qx "lefthook install" "${log}"; then
  fail_case "the hooks" "lefthook was not asked to install them"
fi
count_case
if grep -q '^useradd ' "${log}"; then
  fail_case "a gate user who exists" "useradd ran anyway"
fi

tampered="${workdir}/tampered"
repo "${tampered}"
installed "${tampered}.prefix"
rm "${tampered}.prefix/stamps/shellcheck@0.11.0"
mkdir -p "${workdir}/shellcheck/shellcheck-v0.11.0"
printf '#!/bin/sh\n' >"${workdir}/shellcheck/shellcheck-v0.11.0/shellcheck"
tar -cJf "${served}/shellcheck-v0.11.0.linux.x86_64.tar.xz" -C "${workdir}/shellcheck" shellcheck-v0.11.0
printf 'genuine' >"${workdir}/genuine"
pin_checksum "${tampered}" shellcheck 0.11.0 "${workdir}/genuine"
expect_output 1 "a download that misses its checksum" "does not match the SHA-256 pinned for shellcheck" setup "${tampered}"
count_case
if [[ -e "${tampered}.prefix/stamps/shellcheck@0.11.0" || -e "${tampered}.prefix/bin/shellcheck" ]]; then
  fail_case "a download that misses its checksum" "it was installed anyway"
fi

missing="${workdir}/missing"
repo "${missing}"
installed "${missing}.prefix"
rm "${missing}.prefix/stamps/jq@1.8.2"
expect_output 1 "a download that fails" "jq 1.8.2 was not installed" setup "${missing}"

# Every download is unpacked or written to the place the setup links it from,
# runnable by anyone, as the gate user runs it.
# Run as root, tar keeps the owner an archive names unless told not to; run as
# anyone else it cannot keep one, so only root can tell.
placed="${workdir}/placed"
repo "${placed}"
installed "${placed}.prefix"
rm "${placed}.prefix/stamps/"{node@24.21.0,shellcheck@0.11.0,jq@1.8.2,lefthook@2.2.1}
rm "${placed}.prefix/stamps/"{typos@1.51.1,hadolint@2.15.1,taplo@0.10.0,zizmor@1.30.1}
packed="${workdir}/packed"
# Each pair is where a tool goes under the prefix and where it is in the
# release.
releases='node/bin/node:node-v24.21.0-linux-x64/bin/node
bin/shellcheck:shellcheck-v0.11.0/shellcheck bin/typos:typos/typos
bin/zizmor:zizmor/zizmor bin/jq:jq bin/hadolint:hadolint bin/lefthook:lefthook
bin/taplo:taplo'
for pair in ${releases}; do
  mkdir -p "$(dirname "${packed}/${pair#*:}")"
  printf '#!/bin/sh\n# %s\n' "${pair#*:}" >"${packed}/${pair#*:}"
  chmod 0755 "${packed}/${pair#*:}"
done
if ((EUID == 0)); then chown -R 1001:1001 "${packed}"; fi
tar -cJf "${served}/node-v24.21.0-linux-x64.tar.xz" -C "${packed}" node-v24.21.0-linux-x64
tar -cJf "${served}/shellcheck-v0.11.0.linux.x86_64.tar.xz" -C "${packed}" shellcheck-v0.11.0
tar -czf "${served}/typos-v1.51.1-x86_64-unknown-linux-musl.tar.gz" -C "${packed}/typos" ./typos
tar -czf "${served}/zizmor-x86_64-unknown-linux-gnu.tar.gz" -C "${packed}/zizmor" zizmor
cp "${packed}/jq" "${served}/jq-linux-amd64"
cp "${packed}/hadolint" "${served}/hadolint-Linux-x86_64"
gzip -c "${packed}/lefthook" >"${served}/lefthook_2.2.1_Linux_x86_64.gz"
gzip -c "${packed}/taplo" >"${served}/taplo-linux-x86_64.gz"
pin_checksum "${placed}" node 24.21.0 "${served}/node-v24.21.0-linux-x64.tar.xz"
pin_checksum "${placed}" shellcheck 0.11.0 "${served}/shellcheck-v0.11.0.linux.x86_64.tar.xz"
pin_checksum "${placed}" typos 1.51.1 "${served}/typos-v1.51.1-x86_64-unknown-linux-musl.tar.gz"
pin_checksum "${placed}" zizmor 1.30.1 "${served}/zizmor-x86_64-unknown-linux-gnu.tar.gz"
pin_checksum "${placed}" jq 1.8.2 "${served}/jq-linux-amd64"
pin_checksum "${placed}" hadolint 2.15.1 "${served}/hadolint-Linux-x86_64"
pin_checksum "${placed}" lefthook 2.2.1 "${served}/lefthook_2.2.1_Linux_x86_64.gz"
pin_checksum "${placed}" taplo 0.10.0 "${served}/taplo-linux-x86_64.gz"
expect_exit pass "every download installed" setup "${placed}"
for pair in ${releases}; do
  tool="${pair%%:*}"
  count_case
  if ! cmp -s "${packed}/${pair#*:}" "${placed}.prefix/${tool}" \
    || [[ -z "$(find "${placed}.prefix/${tool}" -perm -o=x)" ]] \
    || [[ "$(readlink "${placed}.bin/${tool##*/}")" != "${placed}.prefix/${tool}" ]]; then
    fail_case "every download installed" "${tool} is not the release's own, runnable by the gate user and linked onto PATH"
  fi
done
if ((EUID == 0)); then
  count_case
  kept="$(find "${placed}.prefix/node" "${placed}.prefix/bin" ! -uid 0)"
  if [[ -n "${kept}" ]]; then
    fail_case "every download installed, as root" "the toolchain kept an archive's owner: ${kept%%$'\n'*}"
  fi
fi

# A tool written over while it runs, as when another lane's git hook is
# running lefthook, fails with "Text file busy"; installed again, it is
# replaced instead.
busy="${workdir}/busy"
repo "${busy}"
installed "${busy}.prefix"
rm "${busy}.prefix/stamps/"{lefthook@2.2.1,taplo@0.10.0}
pin_checksum "${busy}" lefthook 2.2.1 "${served}/lefthook_2.2.1_Linux_x86_64.gz"
pin_checksum "${busy}" taplo 0.10.0 "${served}/taplo-linux-x86_64.gz"
running=()
for tool in lefthook taplo; do
  cp "$(command -v bash)" "${busy}.prefix/bin/${tool}"
  # shellcheck disable=SC2016 # expanded by the running copy
  "${busy}.prefix/bin/${tool}" -c ': >"$1"; while :; do sleep 1; done' _ "${busy}.${tool}-running" &
  running+=("$!")
done
for tool in lefthook taplo; do
  for _ in {1..50}; do
    if [[ -e "${busy}.${tool}-running" ]]; then break; fi
    sleep 0.1
  done
done
expect_exit pass "tools installed again while they run" setup "${busy}"
kill "${running[@]}" 2>/dev/null || true
wait "${running[@]}" 2>/dev/null || true
for stamp in lefthook@2.2.1 taplo@0.10.0; do
  tool="${stamp%@*}"
  count_case
  if [[ ! -e "${busy}.${tool}-running" ]]; then
    fail_case "tools installed again while they run" "${tool} never ran"
  elif ! cmp -s "${packed}/${tool}" "${busy}.prefix/bin/${tool}" || [[ ! -e "${busy}.prefix/stamps/${stamp}" ]]; then
    fail_case "tools installed again while they run" "${tool} was not replaced"
  fi
done

again="${workdir}/again"
repo "${again}"
installed "${again}.prefix"
rm "${again}.prefix/stamps/jq@1.8.2"
printf '#!/bin/sh\n' >"${served}/jq-linux-amd64"
pin_checksum "${again}" jq 1.8.2 "${served}/jq-linux-amd64"
expect_exit pass "an install at exactly the room it needs" setup "${again}" DF_FREE_GB=3
expect_exit pass "the run after an install" setup "${again}" DF_FREE_GB=3
count_case
if grep -qE '^(curl|go|python3|npm|df) ' "${log}"; then
  fail_case "the run after an install" "it fetched or measured again: $(grep -E '^(curl|go|python3|npm|df) ' "${log}" | head -1)"
fi

yamllint_repo="${workdir}/yamllint"
repo "${yamllint_repo}"
installed "${yamllint_repo}.prefix"
rm "${yamllint_repo}.prefix/stamps/yamllint@1.38.0"
expect_exit pass "yamllint, installed again" setup "${yamllint_repo}"
count_case
if ! grep -qE '^pip install .*yamllint==1\.38\.0$' "${log}"; then
  fail_case "yamllint, installed again" "yamllint is not installed at its pin: $(grep '^pip ' "${log}" || true)"
fi
count_case
if [[ "$(readlink "${yamllint_repo}.prefix/bin/yamllint")" != "${yamllint_repo}.prefix/venv/bin/yamllint" ]]; then
  fail_case "yamllint, installed again" "the prefix does not link the venv's yamllint"
fi

unpinned="${workdir}/unpinned"
repo "${unpinned}"
installed "${unpinned}.prefix"
repin "${unpinned}" '/^jq = /d'
expect_output 2 "a tool mise.toml does not pin" "no pin for 'jq'" setup "${unpinned}"

repair="${workdir}/repair"
repo "${repair}"
installed "${repair}.prefix"
rm "${repair}.prefix/stamps/go@1.27.2"
expect_exit pass "a Go already in the prefix" setup "${repair}" GO_ROOT="${repair}.prefix/go"
count_case
if [[ ! -e "${repair}.prefix/go/bin/go" ]]; then
  fail_case "a Go already in the prefix" "the repair removed it"
fi

leftover="${workdir}/leftover"
repo "${leftover}"
installed "${leftover}.prefix"
rm "${leftover}.prefix/stamps/go@1.27.2"
mkdir -p "${leftover}.prefix/go.new/src" "${leftover}.goroot/bin"
printf '#!/bin/sh\n# the pinned Go\n' >"${leftover}.goroot/bin/go"
expect_exit pass "a Go build a run cut short left behind" setup "${leftover}"
count_case
if ! cmp -s "${leftover}.goroot/bin/go" "${leftover}.prefix/go/bin/go" || [[ -e "${leftover}.prefix/go/src" ]]; then
  fail_case "a Go build a run cut short left behind" "the pinned Go was copied into what was left of it"
fi

gobump="${workdir}/gobump"
repo "${gobump}"
installed "${gobump}.prefix"
repin "${gobump}" 's/^go = .*/go = "1.27.3"/'
mkdir -p "${gobump}.goroot/bin"
# The pinned Go makes the build cache it is given, as a real one would.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
printf '#!/usr/bin/env bash\nmkdir -p "${GOCACHE}"\nprintf "pinned-go %%s GOBIN=%%s GOTOOLCHAIN=%%s GOFLAGS=%%s GOCACHE=%%s\\n" "$*" "${GOBIN}" "${GOTOOLCHAIN}" "${GOFLAGS}" "${GOCACHE}" >>"%s"\n' "${log}" \
  >"${gobump}.goroot/bin/go"
chmod +x "${gobump}.goroot/bin/go"
# The module cache keeps a toolchain read-only, and so does a copy that keeps
# its modes; write permission comes back after, so the harness can clean up.
chmod -R a-w "${gobump}.goroot"
expect_exit pass "a Go bump" setup "${gobump}"
chmod -R u+w "${gobump}.goroot"
count_case
if [[ -n "$(find "${gobump}.prefix/go" ! -perm -u=w)" ]]; then
  fail_case "a Go bump" "the copy of the module cache's Go is not writable, so the next bump cannot remove it: $(find "${gobump}.prefix/go" ! -perm -u=w | head -1)"
fi
count_case
if ! grep -qx "toolchain go1.27.3" "${log}"; then
  fail_case "a Go bump" "GOROOT was not asked of go1.27.3: $(grep '^toolchain ' "${log}" || true)"
fi
count_case
if ! grep -q '^pinned-go install github.com/go-task/task/v3/cmd/task@v3.54.0 ' "${log}"; then
  fail_case "a Go bump" "task was not rebuilt with the new Go"
fi
count_case
if ! grep -qF "pinned-go install github.com/go-task/task/v3/cmd/task@v3.54.0 GOBIN=${gobump}.prefix/bin GOTOOLCHAIN=local GOFLAGS= " "${log}"; then
  fail_case "a Go bump" "task was not built into the prefix with the local toolchain and no flags of the environment's: $(grep '^pinned-go install github.com/go-task' "${log}" || true)"
fi
while IFS= read -r cache; do
  count_case
  if [[ -z "${cache}" || -e "${cache}" ]]; then
    fail_case "a Go bump" "the tools were not built in a cache of the setup's own, removed after: ${cache}"
  fi
done < <(sed -n 's/^pinned-go install .* GOCACHE=//p' "${log}")
count_case
if [[ -e "${gobump}.prefix/stamps/task@3.54.0+go1.27.2" ]]; then
  fail_case "a Go bump" "the stamp for the old Go's build was kept"
fi

nodebump="${workdir}/nodebump"
repo "${nodebump}"
installed "${nodebump}.prefix"
repin "${nodebump}" 's/^node = .*/node = "24.22.0"/'
mkdir -p "${workdir}/node/node-v24.22.0-linux-x64/bin"
printf '#!/bin/sh\n' >"${workdir}/node/node-v24.22.0-linux-x64/bin/node"
# The new node's own npm, which knows --min-release-age, as the image's
# older npm does not.
# shellcheck disable=SC2016 # the stand-in's own arguments, expanded when it runs
printf '#!/usr/bin/env bash\nprintf "pinned-npm %%s\\n" "$*" >>"%s"\n' "${log}" \
  >"${workdir}/node/node-v24.22.0-linux-x64/bin/npm"
chmod +x "${workdir}/node/node-v24.22.0-linux-x64/bin/npm"
tar -cJf "${served}/node-v24.22.0-linux-x64.tar.xz" -C "${workdir}/node" node-v24.22.0-linux-x64
pin_checksum "${nodebump}" node 24.22.0 "${served}/node-v24.22.0-linux-x64.tar.xz"
expect_exit pass "a node bump" setup "${nodebump}"
count_case
if ! grep -q '^pinned-npm install -g .*markdownlint-cli2@0.23.3' "${log}"; then
  fail_case "a node bump" "markdownlint-cli2 was not installed again by the new node's npm: $(grep -E '^(pinned-)?npm ' "${log}" || true)"
fi
count_case
if ! grep '^pinned-npm install ' "${log}" | grep -q -- ' --min-release-age=7 '; then
  fail_case "markdownlint-cli2's dependencies" "npm may resolve one released within the week: $(grep -E '^(pinned-)?npm ' "${log}" || true)"
fi

full="${workdir}/full"
repo "${full}"
installed "${full}.prefix"
rm "${full}.prefix/stamps/jq@1.8.2"
expect_output 1 "a disk without room" "needs 3 GB free" setup "${full}" DF_FREE_GB=2
count_case
if grep -q '^curl ' "${log}"; then
  fail_case "a disk without room" "the setup fetched anyway"
fi

arm="${workdir}/arm"
repo "${arm}"
expect_output 2 "a machine it has no downloads for" "aarch64" setup "${arm}" UNAME_M=aarch64

held="${workdir}/held"
repo "${held}"
installed "${held}.prefix"
exec {lock}>"${held}.prefix/.setup.lock"
flock "${lock}"
expect_output 1 "another setup holds the lock" "another setup" setup "${held}" CLOUD_SETUP_LOCK_WAIT=1
exec {lock}>&-

# The lock is taken here first, so the setup is sure to find it held, and
# let go of a second later.
waited="${workdir}/waited"
repo "${waited}"
installed "${waited}.prefix"
exec {lock}>"${waited}.prefix/.setup.lock"
flock "${lock}"
(sleep 1 && flock -u "${lock}") &
holder=$!
expect_output pass "a setup waits for another" "toolchain ready" setup "${waited}" CLOUD_SETUP_LOCK_WAIT=20
wait "${holder}"
exec {lock}>&-

newuser="${workdir}/newuser"
repo "${newuser}"
installed "${newuser}.prefix"
expect_exit pass "a gate user who is missing" setup "${newuser}" ID_STATUS=1
count_case
if ! grep -qE '^useradd (.* )?(--create-home|-m)( .*)? dev$' "${log}"; then
  fail_case "a gate user who is missing" "useradd was not asked to make dev with a home"
fi

nouser="${workdir}/nouser"
repo "${nouser}"
installed "${nouser}.prefix"
expect_output 1 "a gate user who cannot be made" "could not make the gate user dev" \
  setup "${nouser}" ID_STATUS=1 USERADD_STATUS=9

nohooks="${workdir}/nohooks"
repo "${nohooks}"
git -C "${nohooks}" init -q
installed "${nohooks}.prefix"
rm "${nohooks}.prefix/stamps/yamllint@1.38.0"
expect_output 1 "git hooks that cannot be installed" "could not install the git hooks" \
  setup "${nohooks}" LEFTHOOK_STATUS=3
count_case
if grep -q 'was not installed' "${case_output}"; then
  fail_case "git hooks that cannot be installed" "it blamed a tool it installed: $(grep 'was not installed' "${case_output}")"
fi

finish_tests
