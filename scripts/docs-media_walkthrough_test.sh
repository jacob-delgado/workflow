#!/usr/bin/env bash
#
# Tests for the web walkthrough's half of scripts/docs-media.sh. With the
# film there it encodes web-walkthrough.webp and passes it, once the WebP is
# the film its manifest describes: animated, 1280 by 800, looping forever,
# as long as the film within a second either way, with no frame past 250 ms
# that begins where the film moves throughout, though a frame held for a
# step's beat plays as long as it is held, and with each step's keyframe
# there to look at. A WebP of 2 MiB passes and a byte more fails by name. It
# refuses a busy machine, a load past all but one of its processors or a
# gate or Playwright run going, and an ffmpeg with no libwebp_anim; it fails
# when the run filmed nothing, or the encoding fails. The stand-ins of
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
  "ok    web-walkthrough.webp: 164 bytes, within its budget of 2097152" media "${root}" "${all}" "${with_browser}"

webp "${workdir}/at-budget.webp" 1280 800 0 2096980 100 100 100 100 2600
expect_output pass "a WebP exactly at its budget" "web-walkthrough.webp: 2097152 bytes, within its budget of 2097152" \
  media "${root}" "${all}" "${with_browser}" "FAKE_WEBP=${workdir}/at-budget.webp"
webp "${workdir}/past-budget.webp" 1280 800 0 2096982 100 100 100 100 2600
expect_output 1 "a WebP past its budget" "web-walkthrough.webp: 2097154 bytes, past its budget of 2097152" \
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
printf '3.00 2.00 1.00 1/100 1\n' >"${workdir}/full-load"
expect_exit pass "a load of all but one of the machine's processors" media "${root}" "${all}" "${with_browser}" \
  "DOCS_MEDIA_LOADAVG=${workdir}/full-load"
expect_output 2 "a gate going" "a gate or a Playwright run is going" \
  media "${root}" "${all}" "${with_browser}" "FAKE_RUNNING=4242 bash scripts/gate-as-dev.sh ."
expect_output 2 "no load to read" "cannot read the machine's load" \
  media "${root}" "${all}" "${with_browser}" "DOCS_MEDIA_LOADAVG=${workdir}/missing-load"

finish_tests
