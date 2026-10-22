#!/usr/bin/env bash
#
# Tests for the web walkthrough's half of scripts/docs-media.sh. With the
# film there it encodes web-walkthrough.webp and passes it, once the WebP is
# the film its manifest describes: animated, 1280 by 800, looping forever,
# with no frame of 10 ms or less, which a browser plays for 100 ms and the
# failure names, as long as the film within a second either way, with no
# frame playing more than 250 ms of a stretch where the film moves
# throughout, whether the time is lost inside it or into the frame before or
# after it, though a frame held for a step's beat plays as long as it is
# held, with no such stretch the film has no frame of, and with each step's
# keyframe there to look at. A WebP of 4 MiB passes and a byte more fails by
# name. It refuses a busy machine, a load past all but one of its processors
# or a gate or Playwright run going, and an ffmpeg with no libwebp_anim; it
# fails when the run filmed nothing, or the encoding fails. The stand-ins of
# lib/docs-media-fixtures.sh say what they were asked, so nothing is built,
# filmed or encoded.
#
# Usage:
#   scripts/docs-media_walkthrough_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/docs-media.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/docs-media-fixtures.sh
source "${here}/lib/docs-media-fixtures.sh"

root="$(fixture walkthrough)"
expect_output pass "the walkthrough encoded, as long as its film" \
  "ok    web-walkthrough.webp: runs 3.00 s, as the film's 3.00 s do" media "${root}" "${all}" "${with_browser}"
expect_output pass "the walkthrough's keyframes counted, to look at" "ok    web-walkthrough.webp: 1 keyframes" \
  media "${root}" "${all}" "${with_browser}"
expect_output pass "the walkthrough held to its own budget" \
  "ok    web-walkthrough.webp: 164 bytes, within its budget of 4194304" media "${root}" "${all}" "${with_browser}"

webp "${workdir}/at-budget.webp" 1280 800 0 4194132 100 100 100 100 2600
expect_output pass "a WebP exactly at its budget" "web-walkthrough.webp: 4194304 bytes, within its budget of 4194304" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/at-budget.webp"
webp "${workdir}/past-budget.webp" 1280 800 0 4194134 100 100 100 100 2600
expect_output 1 "a WebP past its budget" "web-walkthrough.webp: 4194306 bytes, past its budget of 4194304" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/past-budget.webp"

webp "${workdir}/within.webp" 1280 800 0 0 100 100 100 100 1610
expect_output pass "a WebP within a second of its film" "runs 2.01 s, as the film's 3.00 s do" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/within.webp"
webp "${workdir}/short.webp" 1280 800 0 0 100 100 100 100 1590
expect_output 1 "a WebP more than a second short of its film" "runs 1.99 s where the film's 3.00 s do" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/short.webp"
webp "${workdir}/long.webp" 1280 800 0 0 100 100 100 100 3610
expect_output 1 "a WebP more than a second longer than its film" "runs 4.01 s where the film's 3.00 s do" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/long.webp"

webp "${workdir}/stalled.webp" 1280 800 0 0 300 100 2600
expect_output 1 "a frame lost inside a glide, named by where it plays" \
  "frame 1 at 0.00 s plays 300 ms inside a glide" media "${root}" "${all}" "${with_browser}" \
  "FAKE_WEBP=${workdir}/stalled.webp"
webp "${workdir}/at-stall.webp" 1280 800 0 0 250 50 100 2600
expect_exit pass "a frame of 250 ms inside a glide" media "${root}" "${all}" "${with_browser}" \
  "FAKE_WEBP=${workdir}/at-stall.webp"
webp "${workdir}/run-on.webp" 1280 800 0 0 100 100 100 2700
expect_exit pass "a glide's last frame run on into the beat's hold, as lossless encoding merges the same pixels" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/run-on.webp"

webp "${workdir}/brief.webp" 1280 800 0 0 100 10 190 100 2600
expect_output 1 "a frame a browser holds for 100 ms, named by where it plays" \
  "frame 2 at 0.10 s plays 10 ms, which a browser plays for 100 ms" media "${root}" "${all}" "${with_browser}" \
  "FAKE_WEBP=${workdir}/brief.webp"
webp "${workdir}/past-brief.webp" 1280 800 0 0 100 11 189 100 2600
expect_exit pass "a frame of 11 ms, which a browser plays as long" media "${root}" "${all}" "${with_browser}" \
  "FAKE_WEBP=${workdir}/past-brief.webp"

# A film with a still frame before its glide and a beat after it: the still
# frame plays 600 ms, then twelve glide frames 33 ms apart, the last held
# 1.4 s for its step's beat. The glide moves from 0.60 s to 0.96 s of it.
{
  printf '%s\n' 'size 1280 800' 'length 2396' 'frame frames/00001.png 0 600'
  for glide in $(seq 0 11); do
    printf 'frame frames/%05d.png %d %d\n' "$((glide + 2))" "$((600 + glide * 33))" "$((glide == 11 ? 1433 : 33))"
  done
  printf '%s\n' 'key keys/01.png 1050 PROJ-412 picked' 'stretch 600 970 a glide'
} >"${workdir}/edges.txt"
webp "${workdir}/lost-at-start.webp" 1280 800 0 0 864 33 33 33 1433
expect_output 1 "a glide's first frames lost into the still frame before it" \
  "frame 1 at 0.00 s plays 264 ms inside a glide" media "${root}" "${all}" "${with_browser}" \
  "FAKE_MANIFEST=${workdir}/edges.txt" "FAKE_WEBP=${workdir}/lost-at-start.webp"
webp "${workdir}/lost-at-end.webp" 1280 800 0 0 600 33 33 33 1697
expect_output 1 "a glide's last frames lost into the beat after it" \
  "frame 5 at 0.70 s plays 264 ms inside a glide" media "${root}" "${all}" "${with_browser}" \
  "FAKE_MANIFEST=${workdir}/edges.txt" "FAKE_WEBP=${workdir}/lost-at-end.webp"
webp "${workdir}/first-merged.webp" 1280 800 0 0 633 33 33 33 33 33 33 33 33 33 33 1433
expect_exit pass "a glide's first frame merged with the still frame it matches" \
  media "${root}" "${all}" "${with_browser}" \
  "FAKE_MANIFEST=${workdir}/edges.txt" "FAKE_WEBP=${workdir}/first-merged.webp"
grep -v '^frame frames/0000[2-9]\|^frame frames/0001' "${workdir}/edges.txt" >"${workdir}/unfilmed.txt"
webp "${workdir}/unfilmed.webp" 1280 800 0 0 2396
expect_output 1 "a glide the film has no frame of" "a glide drawn at 0.60 s has no frame in the film" \
  media "${root}" "${all}" "${with_browser}" \
  "FAKE_MANIFEST=${workdir}/unfilmed.txt" "FAKE_WEBP=${workdir}/unfilmed.webp"

webp "${workdir}/canvas.webp" 1440 900 0 0 100 100 100 100 2600
expect_output 1 "a WebP whose canvas is not the film's" "its canvas is not the film's 1280 800" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/canvas.webp"
webp "${workdir}/once.webp" 1280 800 1 0 100 100 100 100 2600
expect_output 1 "a WebP that plays once" "does not loop forever" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/once.webp"
printf 'RIFF\0\0\0\0WEBPVP8 \0\0\0\0' >"${workdir}/still.webp"
expect_output 1 "a WebP that is not animated" "web-walkthrough.webp: not an animated WebP" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/still.webp"
printf 'GIF89a' >"${workdir}/gif.webp"
expect_output 1 "a file that is no WebP" "web-walkthrough.webp: not an animated WebP" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/gif.webp"

expect_output 1 "a keyframe the film names, missing" \
  "the walkthrough's keyframe for \"PROJ-412 picked\" is missing" media "${root}" "${all}" "${with_browser}" \
  FAKE_NO_KEYS=1
grep -v '^key ' "${workdir}/film.txt" >"${workdir}/no-keys.txt"
expect_output 1 "a film that kept no keyframe" "the walkthrough kept no keyframe" \
  media "${root}" "${all}" "${with_browser}" "FAKE_MANIFEST=${workdir}/no-keys.txt"

expect_output pass "every frame the WebP holds the film's own" \
  "ok    web-walkthrough.webp: every frame it holds is the film's" media "${root}" "${all}" "${with_browser}"
expect_output pass "the WebP held to the film by web's media:check, after it is encoded" \
  "corepack yarn media:check in web" media "${root}" "${all}" "${with_browser}"
expect_output 1 "a frame the WebP holds that is not the film's" \
  "web-walkthrough.webp: a frame it holds is not the film's" media "${root}" "${all}" "${with_browser}" \
  FAKE_FIDELITY_FAIL=1

root="$(fixture not-filmed)"
expect_output 1 "a run that filmed nothing" "the walkthrough was not filmed by this run" \
  media "${root}" "${all}" "${with_browser}" FAKE_NO_FILM=1
root="$(fixture stale-film)"
mkdir -p "${root}/tmp/walkthrough"
cp "${workdir}/film.txt" "${root}/tmp/walkthrough/walkthrough.txt"
touch -t 202601010000 "${root}/tmp/walkthrough/walkthrough.txt"
expect_output 1 "a film an earlier run left" "the walkthrough was not filmed by this run" \
  media "${root}" "${all}" "${with_browser}" FAKE_NO_FILM=1
root="$(fixture no-encoding)"
expect_output 1 "the encoding fails" "could not encode the walkthrough" \
  media "${root}" "${all}" "${with_browser}" FAKE_ENCODE_FAIL=1
root="$(fixture stale-webp)"
touch -t 202601010000 "${root}/docs/assets/images/web-intro.webp"
expect_output 1 "a WebP this run did not write" "web-intro.webp: not written by this run" \
  media "${root}" "${all}" "${with_browser}"

root="$(fixture no-webp)"
expect_output 2 "an ffmpeg with no animated WebP encoder" "has no libwebp_anim encoder" \
  media "${root}" "${all}" "${with_browser}" FAKE_NO_WEBP=1

root="$(fixture busy)"
printf '3.01 2.00 1.00 1/100 1\n' >"${workdir}/busy-load"
expect_output 2 "a load past all but one of the machine's processors" \
  "the machine is busy, a load of 3.01 on 4 processors" media "${root}" "${all}" "${with_browser}" \
  "DOCS_MEDIA_LOADAVG=${workdir}/busy-load"

# not_built runs the script and fails when it built VHS, keeping its exit
# status otherwise.
#   not_built <root> <dirs> [NAME=value...]
not_built() {
  local status out="${workdir}/not-built.out"
  set +e
  media "$@" >"${out}" 2>&1
  status=$?
  set -e
  cat "${out}"
  if grep -q 'go install' "${out}"; then
    echo "VHS was built before the machine's load was read"
    exit 9
  fi
  exit "${status}"
}

# The load is read before the script's own setup, building VHS cold among it,
# so the load refused is other work's, not the script's.
root="$(fixture busy-no-vhs)"
expect_output 2 "a busy machine refused before VHS is built" "the machine is busy" \
  not_built "${root}" "stubs curl ttyd ffmpeg" "${with_browser}" "DOCS_MEDIA_LOADAVG=${workdir}/busy-load"
printf '3.00 2.00 1.00 1/100 1\n' >"${workdir}/full-load"
expect_exit pass "a load of all but one of the machine's processors" media "${root}" "${all}" "${with_browser}" \
  "DOCS_MEDIA_LOADAVG=${workdir}/full-load"
expect_output 2 "a gate going" "a gate or a Playwright run is going" \
  media "${root}" "${all}" "${with_browser}" "FAKE_RUNNING=4242 bash scripts/gate-as-dev.sh ."
expect_output 2 "no load to read" "cannot read the machine's load" \
  media "${root}" "${all}" "${with_browser}" "DOCS_MEDIA_LOADAVG=${workdir}/missing-load"

finish_tests
