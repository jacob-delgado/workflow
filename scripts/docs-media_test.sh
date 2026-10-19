#!/usr/bin/env bash
#
# Tests for scripts/docs-media.sh. With every tool there at its pin it builds
# the demo, takes the web screenshots, plays the tape from the repository's
# root and prints each image's size within its budget: a PNG of 300 KiB and a
# GIF of 2 MiB pass, a byte more fails by name, and so does an image this run
# did not write. It films with the Chromium PLAYWRIGHT_CHROMIUM_EXECUTABLE
# names, relative or not, or else the chromium on PATH, linked as `chrome` and
# `chromium`, and refuses to run with neither, as without ffmpeg; it fails
# when VHS fetched a browser into ~/.cache/rod, though not over one an earlier
# run left there; and it asks for Chromium without its sandbox as root alone.
# A VHS missing or at another version is built at the pin, in a build cache
# removed after, whether the build succeeds or not. A ttyd is hashed before
# anything runs it: one on PATH, or behind a mise shim, that matches its
# pinned SHA-256 is used; one that does not is refused by name, never run; a
# copy an earlier run kept that no longer matches is downloaded again; and
# with none, ttyd is downloaded for this machine, x86_64 or aarch64, and must
# match its pin, refused on a machine ttyd has no build for or with no curl.
# Each failed step fails the run by name: the demo's build, VHS's build or
# its version, ttyd's download or its version, the screenshots and the
# recording; and so does a mise.toml that pins no VHS. The GIF must run as
# long as its tape takes shown, its sleeps and its typing at the tape's speed
# or VHS's own, within a second either way; a tape line of no length to read
# fails, and a Wait while shown is refused, though not one while hidden. A
# change git has not committed to the demo, the tape or the web app, or a
# checkout git does not know, stops the run before anything is built; a
# change to a page no image shows does not. Stand-ins for go, corepack, vhs,
# ttyd, mise, ffmpeg, ffprobe, curl, uname and id say what they were asked,
# so nothing is built, filmed or fetched.
#
# Usage:
#   scripts/docs-media_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/docs-media.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly sys_bin="${workdir}/sys"
readonly browser="${workdir}/browsers/chrome-bin"
readonly payload="${workdir}/ttyd-payload"
readonly ran_marker="${workdir}/unverified-ttyd-ran"
mkdir -p "${sys_bin}" "${workdir}/browsers"

# The system's own tools the script and the stand-ins use, alone on PATH, so
# no real ffmpeg, Chromium, VHS, ttyd or mise a machine has is found.
for tool in bash env sed awk cat mkdir ln rm touch find wc chmod mv cp head readlink dirname basename grep \
  sha256sum shasum git; do
  if command -v "${tool}" >/dev/null; then
    ln -s "$(command -v "${tool}")" "${sys_bin}/${tool}"
  fi
done

# stub writes a stand-in command into a directory of its own.
#   stub <dir> <name> <body>
stub() {
  mkdir -p "${workdir}/$1"
  printf '#!/usr/bin/env bash\n%s\n' "$3" >"${workdir}/$1/$2"
  chmod +x "${workdir}/$1/$2"
}

# The stand-ins every case has: go builds the demo, or fails to as
# FAKE_GO_FAIL asks, and installs the VHS stand-in into GOBIN, or fails to,
# leaving its build cache, as FAKE_INSTALL_FAIL asks; corepack writes each
# screenshot FAKE_PNG_BYTES long, or fails as FAKE_COREPACK_FAIL asks; uname
# and id answer as FAKE_SYSTEM, FAKE_MACHINE and FAKE_UID say.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub stubs go 'case "$1" in
  build) [[ -z "${FAKE_GO_FAIL:-}" ]] || exit 1; touch "$3" ;;
  install)
    echo "go install $2 into ${GOBIN}"
    mkdir -p "${GOCACHE}"
    [[ -z "${FAKE_INSTALL_FAIL:-}" ]] || exit 1
    cp "'"${workdir}"'/vhs/vhs" "${GOBIN}/vhs" ;;
esac'
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub stubs corepack 'echo "corepack $* in ${PWD##*/}"
[[ -z "${FAKE_COREPACK_FAIL:-}" ]] || exit 1
for screen in issues branch review announcement summary tasks; do
  head -c "${FAKE_PNG_BYTES:-1000}" /dev/zero >"../docs/assets/images/web-${screen}.png"
done'
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub stubs uname 'case "$1" in -s) echo "${FAKE_SYSTEM:-Linux}" ;; -m) echo "${FAKE_MACHINE:-x86_64}" ;; esac'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub stubs id 'echo "${FAKE_UID:-1000}"'
# curl downloads the ttyd payload, or fails as FAKE_CURL_FAIL asks.
# shellcheck disable=SC2016 # the stand-in's own arguments, expanded when it runs
stub curl curl 'echo "curl $2"
[[ -z "${FAKE_CURL_FAIL:-}" ]] || exit 22
cp "'"${payload}"'" "$4"'
# vhs reports the pinned version, or FAKE_VHS_VERSION; asked to record, it
# says what it was asked and with which browser, fails as FAKE_VHS_FAIL asks,
# fetches one of its own when FAKE_ROD asks, and writes the GIF FAKE_GIF_BYTES
# long.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub vhs vhs 'if [[ "$1" == --version ]]; then
  echo "vhs version v${FAKE_VHS_VERSION:-0.12.1} (stand-in)"
  exit 0
fi
echo "vhs $* in ${PWD##*/}"
echo "VHS_NO_SANDBOX=${VHS_NO_SANDBOX:-unset}"
echo "chrome is $(readlink "$(command -v chrome)"), chromium is $(readlink "$(command -v chromium)")"
[[ -z "${FAKE_VHS_FAIL:-}" ]] || exit 1
[[ -z "${FAKE_ROD:-}" ]] || mkdir -p "${HOME}/.cache/rod/browser/chromium-1"
head -c "${FAKE_GIF_BYTES:-1000}" /dev/zero >docs/assets/images/terminal.gif'
# An unverified ttyd at the pin: it says it is 1.7.7, but its bytes are not
# the release's, and running it leaves a mark.
stub other-ttyd ttyd 'touch "'"${ran_marker}"'"
echo "ttyd version 1.7.7-stand-in"'
stub ffmpeg ffmpeg 'exit 0'
stub ffmpeg-alone ffmpeg 'exit 0'
# ffmpeg's ffprobe says the GIF runs FAKE_GIF_SECONDS, or as long as the
# fixture's tape takes.
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub ffmpeg ffprobe 'echo "${FAKE_GIF_SECONDS:-3.400000}"'
stub chromium chromium 'exit 0'
stub old-vhs vhs 'echo "vhs version v0.11.0 (stand-in)"'
printf '#!/usr/bin/env bash\nexit 0\n' >"${browser}"
# The release's ttyd, as far as these tests go: the fixture pins its hash. It
# reports the pinned version, or FAKE_DOWNLOADED_VERSION.
# shellcheck disable=SC2016 # the payload's own variable, expanded when it runs
printf '#!/usr/bin/env bash\necho "ttyd version ${FAKE_DOWNLOADED_VERSION:-1.7.7}-downloaded"\n' >"${payload}"
chmod +x "${browser}" "${payload}"
# The release's ttyd already on PATH, as mise installs it.
mkdir -p "${workdir}/ttyd"
cp "${payload}" "${workdir}/ttyd/ttyd"
# A mise shim for ttyd: a link to mise, which names the file it runs as
# FAKE_MISE_TTYD says.
# shellcheck disable=SC2016 # the stand-in's own arguments, expanded when it runs
stub shim mise '[[ "$1 $2" == "which ttyd" ]] || exit 1
echo "${FAKE_MISE_TTYD}"'
ln -s mise "${workdir}/shim/ttyd"

if command -v sha256sum >/dev/null; then
  payload_sha="$(sha256sum "${payload}")"
else
  payload_sha="$(shasum -a 256 "${payload}")"
fi
readonly payload_sha="${payload_sha%% *}"

# The fixture's tape. Shown, it takes 3.4 seconds: two seconds' sleep, "ab"
# and an enter at its typing speed of 100 ms a key, "cd" at 200 ms a key, two
# downs, and half a second's sleep written as a bare number; what it does
# hidden, its typing and its wait, takes none of the GIF's time.
readonly tape='Output docs/assets/images/terminal.gif
Set TypingSpeed 100ms

Hide
Type "exec tmp/demo"
Enter
Wait+Screen /Issues/
Sleep 1.5s
Show
# A comment takes no time.
Sleep 2s
Type "ab"
Enter
Type@200ms "cd"
Down 2
Sleep 0.5'

# fixture makes a repository holding what the script reads, committed: the
# demo, mise.toml's pins, the tape, ttyd's checksums for both machines, the
# images' directory and web/.
#   fixture <name> [checksum]
fixture() {
  local root="${workdir}/$1"
  mkdir -p "${root}/cmd/demo" "${root}/docs/media" "${root}/docs/assets/images" "${root}/docs/content" \
    "${root}/web" "${root}.home"
  printf 'package main\n' >"${root}/cmd/demo/main.go"
  printf '[tools]\nvhs = "0.12.1"\nttyd = { version = "1.7.7", os = ["linux"] }\n' >"${root}/mise.toml"
  printf '%s\n' "${tape}" >"${root}/docs/media/terminal.tape"
  printf 'ttyd 1.7.7 x86_64 %s\nttyd 1.7.7 aarch64 %s\n' "${2:-${payload_sha}}" "${2:-${payload_sha}}" \
    >"${root}/docs/media/tool-checksums.txt"
  printf '# Usage\n' >"${root}/docs/content/usage.md"
  git -C "${root}" init -q
  git -C "${root}" add .
  git -C "${root}" -c user.name=fixture -c user.email=fixture@example.com commit -qm fixture
  printf '%s' "${root}"
}

# media runs the script over a fixture on a PATH of the stand-in directories
# named and the system's tools, with the environment given besides.
#   media <root> <dirs> [NAME=value...]
media() {
  local root="$1" dirs="$2" path="" dir
  shift 2
  for dir in ${dirs}; do
    path+="${workdir}/${dir}:"
  done
  env -u PLAYWRIGHT_CHROMIUM_EXECUTABLE -u VHS_NO_SANDBOX PATH="${path}${sys_bin}" HOME="${root}.home" \
    DOCS_MEDIA_ROOT="${root}" "$@" "${script}"
}

readonly all="stubs curl vhs ttyd ffmpeg"
readonly with_browser="PLAYWRIGHT_CHROMIUM_EXECUTABLE=${browser}"

root="$(fixture pass)"
expect_output pass "every image written within its budget" "ok    terminal.gif: 1000 bytes, within its budget of 2097152" \
  media "${root}" "${all}" "${with_browser}"
expect_output pass "the demo built, the web captured, the tape played from the root" \
  "corepack yarn media in web
vhs docs/media/terminal.tape in pass" media "${root}" "${all}" "${with_browser}"
expect_output pass "VHS films with the Chromium named, under both names go-rod tries first" \
  "chrome is ${browser}, chromium is ${browser}" media "${root}" "${all}" "${with_browser}"
expect_output pass "a Chromium named by a relative path, linked by its absolute one" \
  "chrome is ${browser}, chromium is ${browser}" \
  run_in "${workdir}" media "${root}" "${all}" "PLAYWRIGHT_CHROMIUM_EXECUTABLE=browsers/chrome-bin"
expect_output pass "Chromium keeps its sandbox when not run as root" "VHS_NO_SANDBOX=unset" \
  media "${root}" "${all}" "${with_browser}"
expect_output pass "Chromium runs without its sandbox as root" "VHS_NO_SANDBOX=true" \
  media "${root}" "${all}" "${with_browser}" FAKE_UID=0
expect_output pass "the chromium on PATH, with no Chromium named" \
  "chrome is ${workdir}/chromium/chromium, chromium is ${workdir}/chromium/chromium" \
  media "${root}" "${all} chromium"

expect_output 2 "no Chromium to film with" "no Chromium" media "${root}" "${all}"
expect_output 2 "a Chromium named that is not there" "no Chromium" \
  media "${root}" "${all}" "PLAYWRIGHT_CHROMIUM_EXECUTABLE=${workdir}/browsers/missing"
expect_output 2 "no ffmpeg" "no ffmpeg on PATH" media "${root}" "stubs curl vhs ttyd" "${with_browser}"
expect_output 2 "an ffmpeg without its ffprobe" "no ffprobe on PATH" \
  media "${root}" "stubs curl vhs ttyd ffmpeg-alone" "${with_browser}"
expect_output 2 "an argument" "Usage:" "${script}" extra

root="$(fixture at-budget)"
expect_output pass "a PNG exactly at its budget" "web-issues.png: 307200 bytes, within its budget of 307200" \
  media "${root}" "${all}" "${with_browser}" FAKE_PNG_BYTES=307200
expect_output 1 "a PNG past its budget" "web-issues.png: 307201 bytes, past its budget of 307200" \
  media "${root}" "${all}" "${with_browser}" FAKE_PNG_BYTES=307201
expect_output pass "a GIF exactly at its budget" "terminal.gif: 2097152 bytes, within its budget of 2097152" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_BYTES=2097152
expect_output 1 "a GIF past its budget" "terminal.gif: 2097153 bytes, past its budget of 2097152" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_BYTES=2097153

root="$(fixture length)"
expect_output pass "a GIF as long as its tape's shown part takes" \
  "ok    terminal.gif: runs 3.40 s, as its tape's 3.40 s do" media "${root}" "${all}" "${with_browser}"
expect_output pass "a GIF within a second of its tape" "ok    terminal.gif: runs 2.41 s" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_SECONDS=2.410000
expect_output 1 "a GIF more than a second short of its tape, as when VHS lost frames" \
  "terminal.gif: runs 2.39 s where its tape's 3.40 s do" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_SECONDS=2.390000
expect_output 1 "a GIF more than a second longer than its tape" "terminal.gif: runs 4.41 s where its tape's 3.40 s do" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_SECONDS=4.410000

root="$(fixture waits)"
printf 'Wait+Screen /announced/\n' >>"${root}/docs/media/terminal.tape"
git -C "${root}" -c user.name=fixture -c user.email=fixture@example.com commit -qam "wait shown"
expect_output 2 "a tape that waits on the screen while shown, whose length cannot be known" \
  "docs/media/terminal.tape waits on the screen while it is shown" media "${root}" "${all}" "${with_browser}"

root="$(fixture unreadable)"
printf 'Sleep a while\n' >>"${root}/docs/media/terminal.tape"
git -C "${root}" -c user.name=fixture -c user.email=fixture@example.com commit -qam "a sleep of no length"
expect_output 1 "a tape line whose length cannot be read" "cannot read how long this takes: Sleep a while" \
  media "${root}" "${all}" "${with_browser}"

root="$(fixture typing-speed)"
sed -i.orig '/^Set TypingSpeed/d' "${root}/docs/media/terminal.tape"
rm "${root}/docs/media/terminal.tape.orig"
git -C "${root}" -c user.name=fixture -c user.email=fixture@example.com commit -qam "VHS's own typing speed"
expect_output pass "a tape at VHS's own typing speed of 50 ms a key" "runs 3.15 s, as its tape's 3.15 s do" \
  media "${root}" "${all}" "${with_browser}" FAKE_GIF_SECONDS=3.150000

root="$(fixture changed-demo)"
printf '// changed\n' >>"${root}/cmd/demo/main.go"
expect_output 2 "a change to the demo not committed" "M cmd/demo/main.go" media "${root}" "${all}" "${with_browser}"
root="$(fixture changed-tape)"
printf 'Sleep 1s\n' >>"${root}/docs/media/terminal.tape"
expect_output 2 "a change to the tape not committed" "not committed" media "${root}" "${all}" "${with_browser}"
root="$(fixture new-web-file)"
printf 'export {}\n' >"${root}/web/screen.ts"
expect_output 2 "a web file not committed" "?? web/screen.ts" media "${root}" "${all}" "${with_browser}"
root="$(fixture changed-page)"
printf 'More.\n' >>"${root}/docs/content/usage.md"
expect_exit pass "a change to a page, which no image shows" media "${root}" "${all}" "${with_browser}"
root="$(fixture not-a-checkout)"
rm -rf "${root}/.git"
expect_output 2 "a checkout git does not know" "is not a git checkout" media "${root}" "${all}" "${with_browser}"

root="$(fixture stale)"
touch -t 202601010000 "${root}/docs/assets/images/web-settings.png"
expect_output 1 "an image this run did not write" "web-settings.png: not written by this run" \
  media "${root}" "${all}" "${with_browser}"

root="$(fixture rod)"
expect_output 1 "VHS fetched a browser of its own" "VHS downloaded a browser into ${root}.home/.cache/rod" \
  media "${root}" "${all}" "${with_browser}" FAKE_ROD=1

root="$(fixture old-rod)"
mkdir -p "${root}.home/.cache/rod/browser/chromium-0"
touch -t 202601010000 "${root}.home/.cache/rod/browser/chromium-0" "${root}.home/.cache/rod/browser" \
  "${root}.home/.cache/rod"
expect_exit pass "a browser an earlier run left in ~/.cache/rod" media "${root}" "${all}" "${with_browser}"

# cache_removed runs the script and checks that a VHS it built, or failed to,
# left no build cache behind, keeping the script's exit status.
#   cache_removed <root> <dirs> [NAME=value...]
cache_removed() {
  local status
  set +e
  media "$@"
  status=$?
  set -e
  if [[ -e "$1/tmp/docs-media/go-build" ]]; then
    echo "the build cache is left in $1/tmp/docs-media/go-build"
    exit 9
  fi
  exit "${status}"
}

root="$(fixture no-vhs)"
expect_output pass "no VHS, built at the pin in a cache removed after" \
  "go install github.com/charmbracelet/vhs@v0.12.1 into ${root}/tmp/docs-media/bin" \
  cache_removed "${root}" "stubs curl ttyd ffmpeg" "${with_browser}"
root="$(fixture old-vhs)"
expect_output pass "a VHS at another version, built at the pin" \
  "go install github.com/charmbracelet/vhs@v0.12.1 into ${root}/tmp/docs-media/bin" \
  cache_removed "${root}" "stubs curl old-vhs ttyd ffmpeg" "${with_browser}"
root="$(fixture vhs-build-fails)"
expect_output 1 "VHS that does not build, its cache removed" "could not build VHS v0.12.1" \
  cache_removed "${root}" "stubs curl ttyd ffmpeg" "${with_browser}" FAKE_INSTALL_FAIL=1
root="$(fixture vhs-built-wrong)"
expect_output 1 "a VHS built that reports another version" "the VHS on PATH is not v0.12.1" \
  media "${root}" "stubs curl ttyd ffmpeg" "${with_browser}" FAKE_VHS_VERSION=0.11.0

# not_downloaded runs the script and fails when it downloaded ttyd, keeping
# its exit status otherwise.
#   not_downloaded <root> <dirs> [NAME=value...]
not_downloaded() {
  local status out="${workdir}/not-downloaded.out"
  set +e
  media "$@" >"${out}" 2>&1
  status=$?
  set -e
  cat "${out}"
  if grep -q 'downloading ttyd' "${out}" || [[ -e "$1/tmp/docs-media/bin/ttyd" ]]; then
    echo "ttyd was downloaded although one that matches its pin is on PATH"
    exit 9
  fi
  exit "${status}"
}

# never_ran runs the script and fails when the unverified ttyd ran, keeping
# the script's exit status otherwise.
#   never_ran <root> <dirs> [NAME=value...]
never_ran() {
  local status
  rm -f "${ran_marker}"
  set +e
  media "$@"
  status=$?
  set -e
  if [[ -e "${ran_marker}" ]]; then
    echo "the unverified ttyd ran"
    exit 9
  fi
  exit "${status}"
}

root="$(fixture verified-ttyd)"
expect_output pass "a ttyd on PATH that matches its pin, used as it is" "ok    terminal.gif" \
  not_downloaded "${root}" "${all}" "${with_browser}"
root="$(fixture shimmed-ttyd)"
expect_output pass "a ttyd behind a mise shim, hashed as the file mise runs, and used" "ok    terminal.gif" \
  not_downloaded "${root}" "stubs curl vhs shim ffmpeg" "${with_browser}" "FAKE_MISE_TTYD=${workdir}/ttyd/ttyd"
root="$(fixture unverified-ttyd)"
expect_output 1 "a ttyd at the pin on PATH that does not match it, refused by name and never run" \
  "the ttyd on PATH, ${workdir}/other-ttyd/ttyd, is not ttyd 1.7.7 as released" \
  never_ran "${root}" "stubs curl vhs other-ttyd ffmpeg" "${with_browser}"
root="$(fixture unverified-shimmed-ttyd)"
expect_output 1 "a mise shim to a ttyd that does not match, refused naming the file mise runs" \
  "the ttyd on PATH, ${workdir}/other-ttyd/ttyd, is not" \
  never_ran "${root}" "stubs curl vhs shim ffmpeg" "${with_browser}" "FAKE_MISE_TTYD=${workdir}/other-ttyd/ttyd"
root="$(fixture tampered-ttyd)"
mkdir -p "${root}/tmp/docs-media/bin"
cp "${workdir}/other-ttyd/ttyd" "${root}/tmp/docs-media/bin/ttyd"
expect_output pass "a ttyd an earlier run kept that no longer matches, never run and downloaded again" \
  "downloading ttyd 1.7.7" never_ran "${root}" "stubs curl vhs ffmpeg" "${with_browser}"

root="$(fixture no-ttyd)"
expect_output pass "no ttyd, downloaded for this machine and checked" \
  "curl https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64" \
  media "${root}" "stubs curl vhs ffmpeg" "${with_browser}"
root="$(fixture arm-ttyd)"
expect_output pass "no ttyd on an aarch64 machine, downloaded for it" \
  "curl https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.aarch64" \
  media "${root}" "stubs curl vhs ffmpeg" "${with_browser}" FAKE_MACHINE=aarch64
root="$(fixture no-curl)"
expect_output 2 "no ttyd and no curl to download it with" "no curl to download ttyd with" \
  media "${root}" "stubs vhs ffmpeg" "${with_browser}"

# not_kept runs the script and fails when a ttyd download it refused was
# kept, keeping the script's exit status otherwise.
#   not_kept <root> <dirs> [NAME=value...]
not_kept() {
  local status
  set +e
  media "$@"
  status=$?
  set -e
  if [[ -e "$1/tmp/docs-media/bin/ttyd" || -e "$1/tmp/docs-media/bin/ttyd.download" ]]; then
    echo "the download is kept in $1/tmp/docs-media/bin"
    exit 9
  fi
  exit "${status}"
}

root="$(fixture bad-ttyd "$(printf '0%.0s' {1..64})")"
expect_output 1 "a ttyd download that does not match its pin, not kept" "does not match the SHA-256 pinned" \
  not_kept "${root}" "stubs curl vhs ffmpeg" "${with_browser}"
root="$(fixture failed-download)"
expect_output 1 "a ttyd download that fails, not kept" "could not download ttyd 1.7.7" \
  not_kept "${root}" "stubs curl vhs ffmpeg" "${with_browser}" FAKE_CURL_FAIL=1
root="$(fixture wrong-ttyd)"
expect_output 1 "a ttyd downloaded that reports another version" "the ttyd on PATH is not 1.7.7" \
  media "${root}" "stubs curl vhs ffmpeg" "${with_browser}" FAKE_DOWNLOADED_VERSION=1.7.6
root="$(fixture longer-ttyd)"
expect_output 1 "a ttyd downloaded at a longer version that begins with the pin" "the ttyd on PATH is not 1.7.7" \
  media "${root}" "stubs curl vhs ffmpeg" "${with_browser}" FAKE_DOWNLOADED_VERSION=1.7.70
root="$(fixture mac)"
expect_output 2 "ttyd needed on a machine it has no build for" "ttyd 1.7.7 publishes no build for Darwin arm64" \
  media "${root}" "stubs curl vhs ffmpeg" "${with_browser}" FAKE_SYSTEM=Darwin FAKE_MACHINE=arm64

root="$(fixture no-demo)"
expect_output 1 "the demo does not build" "could not build cmd/demo" \
  media "${root}" "${all}" "${with_browser}" FAKE_GO_FAIL=1
root="$(fixture no-screenshots)"
expect_output 1 "the web screenshots fail" "could not take the web screenshots" \
  media "${root}" "${all}" "${with_browser}" FAKE_COREPACK_FAIL=1
root="$(fixture no-recording)"
expect_output 1 "VHS fails to record" "VHS could not record docs/media/terminal.tape" \
  media "${root}" "${all}" "${with_browser}" FAKE_VHS_FAIL=1
root="$(fixture unpinned)"
printf '[tools]\nttyd = "1.7.7"\n' >"${root}/mise.toml"
expect_output 2 "mise.toml pins no VHS" "mise.toml pins no version of vhs" \
  media "${root}" "${all}" "${with_browser}"

finish_tests
