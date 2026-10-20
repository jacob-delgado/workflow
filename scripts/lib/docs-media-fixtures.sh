# shellcheck shell=bash
# shellcheck disable=SC2034,SC2154 # workdir and script are the sourcing test's, and all and with_browser its to use
#
# The fixtures scripts/docs-media_test.sh and docs-media_walkthrough_test.sh
# share, sourced after lib/testing.sh: the system's tools alone on PATH,
# stand-ins for every tool the script runs that say what they were asked,
# a committed repository holding what the script reads, and media, which runs
# the script over one.

readonly sys_bin="${workdir}/sys"
readonly browser="${workdir}/browsers/chrome-bin"
readonly payload="${workdir}/ttyd-payload"
readonly ran_marker="${workdir}/unverified-ttyd-ran"
mkdir -p "${sys_bin}" "${workdir}/browsers"

# The system's own tools the script and the stand-ins use, alone on PATH, so
# no real ffmpeg, Chromium, VHS, ttyd or mise a machine has is found.
for tool in bash env sed awk cat mkdir ln rm touch find wc chmod mv cp head readlink dirname basename grep \
  sha256sum shasum git od tr; do
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
# screenshot FAKE_PNG_BYTES long, or fails as FAKE_COREPACK_FAIL asks, and
# finds the WebP the film's, or fails to as FAKE_FIDELITY_FAIL asks; uname
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
if [[ "$*" == *media:check* ]]; then
  [[ -z "${FAKE_FIDELITY_FAIL:-}" ]] || exit 1
  exit 0
fi
[[ -z "${FAKE_COREPACK_FAIL:-}" ]] || exit 1
for screen in issues branch review announcement summary tasks; do
  head -c "${FAKE_PNG_BYTES:-1000}" /dev/zero >"../docs/assets/images/web-${screen}.png"
done
[[ -z "${FAKE_NO_FILM:-}" ]] || exit 0
rm -rf ../tmp/walkthrough
mkdir -p ../tmp/walkthrough/keys
cp "${FAKE_MANIFEST:-'"${workdir}"'/film.txt}" ../tmp/walkthrough/walkthrough.txt
echo "ffconcat version 1.0" >../tmp/walkthrough/walkthrough.ffconcat
[[ -n "${FAKE_NO_KEYS:-}" ]] || touch ../tmp/walkthrough/keys/01.png'
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
stub stubs uname 'case "$1" in -s) echo "${FAKE_SYSTEM:-Linux}" ;; -m) echo "${FAKE_MACHINE:-x86_64}" ;; esac'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub stubs id 'echo "${FAKE_UID:-1000}"'
# getconf counts FAKE_CPUS processors, four by default; pgrep finds the run
# FAKE_RUNNING names, and none by default.
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub stubs getconf 'echo "${FAKE_CPUS:-4}"'
# shellcheck disable=SC2016 # the stand-in's own variable, expanded when it runs
stub stubs pgrep '[[ -n "${FAKE_RUNNING:-}" ]] || exit 1
echo "${FAKE_RUNNING}"'
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
# ffmpeg lists its libwebp_anim encoder, unless FAKE_NO_WEBP asks it not to,
# and encodes the walkthrough by writing FAKE_WEBP, the fixture's WebP by
# default, as its last argument, or fails as FAKE_ENCODE_FAIL asks.
# shellcheck disable=SC2016 # the stand-in's own variables, expanded when it runs
readonly fake_ffmpeg='if [[ "$*" == *-encoders* ]]; then
  [[ -n "${FAKE_NO_WEBP:-}" ]] || echo " V....D libwebp_anim         libwebp WebP image (codec webp)"
  exit 0
fi
[[ -z "${FAKE_ENCODE_FAIL:-}" ]] || exit 1
cp "${FAKE_WEBP:-'"${workdir}"'/film.webp}" "${!#}"'
stub ffmpeg ffmpeg "${fake_ffmpeg}"
stub ffmpeg-alone ffmpeg "${fake_ffmpeg}"
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
    DOCS_MEDIA_ROOT="${root}" DOCS_MEDIA_LOADAVG="${workdir}/loadavg" "$@" "${script}"
}

readonly all="stubs curl vhs ttyd ffmpeg"
readonly with_browser="PLAYWRIGHT_CHROMIUM_EXECUTABLE=${browser}"

# le prints a number as the little-endian bytes of the width given, as
# printf escapes.
#   le <number> <bytes>
le() {
  local index
  for ((index = 0; index < $2; index++)); do
    printf '\\x%02x' "$((($1 >> (8 * index)) & 255))"
  done
}

# webp writes an animated WebP's chunks, as far as the script reads them: a
# canvas, a loop count and a frame of each duration given, with no image
# data, then a JUNK chunk of the even number of bytes given, to make the
# file as large as a case needs.
#   webp <file> <width> <height> <loop> <padding> <ms>...
webp() {
  local file="$1" width="$2" height="$3" loop="$4" padding="$5" body ms
  shift 5
  body="VP8X$(le 10 4)$(le 2 1)$(le 0 3)$(le "$((width - 1))" 3)$(le "$((height - 1))" 3)"
  body+="ANIM$(le 6 4)$(le 0 4)$(le "${loop}" 2)"
  for ms in "$@"; do
    body+="ANMF$(le 16 4)$(le 0 12)$(le "${ms}" 3)$(le 0 1)"
  done
  {
    printf "RIFF%bWEBP" "$(le 0 4)"
    printf '%b' "${body}"
    if ((padding > 0)); then
      printf "JUNK%b" "$(le "${padding}" 4)"
      head -c "${padding}" /dev/zero
    fi
  } >"${file}"
}

# The fixture's film: five frames, the first four a glide, the last held for
# its step's beat, and the WebP that encodes it.
printf '%s\n' 'size 1280 800' 'length 3000' 'frame frames/00001.png 0 100' 'frame frames/00002.png 100 100' \
  'frame frames/00003.png 200 100' 'frame frames/00004.png 300 100' 'frame frames/00005.png 400 2600' \
  'key keys/01.png 450 PROJ-412 picked' 'stretch 0 350 a glide' >"${workdir}/film.txt"
webp "${workdir}/film.webp" 1280 800 0 0 100 100 100 100 2600
# An idle machine's load: well within four processors.
printf '0.50 0.40 0.30 1/100 1\n' >"${workdir}/loadavg"
