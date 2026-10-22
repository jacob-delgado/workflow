# shellcheck shell=bash
# shellcheck disable=SC2154 # root, images and started are docs-media.sh's
#
# The web walkthrough's half of scripts/docs-media.sh, sourced by it after
# lib/webp.sh: refusing to film on a busy machine, encoding the film
# web/e2e/docs/walkthrough.spec.ts leaves in tmp/walkthrough into an animated
# WebP, and holding the WebP to the film's manifest, walkthrough.txt, whose
# lines film.ts writes: "size <width> <height>", "length <ms>", then a
# "frame <file> <drawn at> <plays for>", "key <file> <at> <caption>" and
# "stretch <from> <to> <what>" for each frame, keyframe and stretch that moves
# throughout, every time in milliseconds on the film's clock. It uses the
# script's refuse, fail and seconds, and its root, images and started.

# stall_ms is the longest a WebP frame may play inside a stretch that moves
# throughout before it counts as frames the encoding lost.
readonly stall_ms=250

# brief_ms is the most a WebP frame may play and still be played as long: a
# browser shows a frame of 10 ms or less for 100 ms, as it does a GIF's.
readonly brief_ms=10

# refuse_busy refuses to film while the machine is busy with something else:
# its load over the last minute past all but one of its processors, or a gate
# or a Playwright run going. A busy machine draws frames late, and a late
# frame is a jump in the film. It runs before the script's own setup, whose
# cold build of VHS would count as the machine being busy. DOCS_MEDIA_LOADAVG
# names the load average to read, /proc/loadavg by default.
refuse_busy() {
  local load cpus running
  read -r load _ <"${DOCS_MEDIA_LOADAVG:-/proc/loadavg}" || refuse "cannot read the machine's load"
  cpus="$(getconf _NPROCESSORS_ONLN)"
  if awk -v average="${load}" -v cpus="${cpus}" 'BEGIN { exit !(average > cpus - 1) }'; then
    refuse "the machine is busy, a load of ${load} on ${cpus} processors, and would film the walkthrough late: run it again once the machine is idle"
  fi
  running="$(pgrep -fa -- 'gate-as-dev[.]sh|task (check|e2e)|playwright test' || true)"
  if [[ -n "${running}" ]]; then
    refuse "a gate or a Playwright run is going, and would film the walkthrough late: run it again once it ends:
${running}"
  fi
}

# refuse_no_webp refuses to run with an ffmpeg that cannot write an
# animated WebP.
refuse_no_webp() {
  local encoders
  encoders="$(ffmpeg -hide_banner -encoders 2>/dev/null || true)"
  [[ "${encoders}" =~ [[:space:]]libwebp_anim[[:space:]] ]] \
    || refuse "the ffmpeg on PATH has no libwebp_anim encoder to write the walkthrough's WebP with"
}

# encode_walkthrough encodes the film into web-walkthrough.webp, lossless and
# looping forever, each frame for as long as the film plays it. The film is
# PNG, so a frame's unchanged pixels are the last frame's and the encoding
# keeps only what moved; a lossy encoding left blocks of earlier frames on
# screen. passthrough keeps every frame at its own time, where ffmpeg would
# otherwise fit them to 25 a second and drop the frame a step settles on;
# the film's list puts each PNG on a clock of a millisecond for the same
# reason, since the concat demuxer reads an image at 25 a second.
encode_walkthrough() {
  local film="${root}/tmp/walkthrough"
  [[ -n "$(find "${film}/walkthrough.txt" -newer "${started}" -print 2>/dev/null)" ]] \
    || fail "the walkthrough was not filmed by this run: ${film}/walkthrough.txt is missing or older"
  (
    cd "${film}"
    ffmpeg -y -v error -f concat -safe 0 -i walkthrough.ffconcat -fps_mode passthrough -c:v libwebp_anim \
      -lossless 1 -compression_level 4 -loop 0 "${images}/web-walkthrough.webp"
  ) || fail "could not encode the walkthrough"
}

# manifest_value prints the value of a manifest line, the words after its
# kind.
#   manifest_value <kind>
manifest_value() {
  awk -v kind="$1" '$1 == kind { $1 = ""; sub(/^ /, ""); print; exit }' "${root}/tmp/walkthrough/walkthrough.txt"
}

# stalls prints each WebP frame that plays past stall_ms of a stretch where
# the film moves throughout, a glide, typing or a scroll, and each such
# stretch the film has no frame of: each stretch's frames are mapped from when
# the browser drew them to when the film plays them, from the first one's
# start to the last one's, and the share of a WebP frame that falls inside
# that span is time the encoding lost once it plays long. A frame lost into
# the still frame before a stretch, or into the beat after it, is counted
# for the part it covers; the last frame running on past a stretch's end, or
# the first merged with the still frame it matches, is the same pixels and
# covers none of it, or less than a frame's worth.
#   stalls <facts>
stalls() {
  awk -v limit="${stall_ms}" '
    FNR == NR {
      if ($1 == "frame") { n++; at[n] = $3; played[n] = $4 }
      if ($1 == "stretch") {
        m++; from[m] = $2; to[m] = $3; what = $4
        for (i = 5; i <= NF; i++) what = what " " $i
        name[m] = what
      }
      next
    }
    $1 == "frame" { k++; lasts[k] = $2 }
    END {
      t = 0
      for (i = 1; i <= n; i++) { film[i] = t; t += played[i] }
      for (s = 1; s <= m; s++) {
        begins[s] = -1; ends[s] = -1
        for (i = 1; i <= n; i++) {
          if (at[i] >= from[s] && at[i] <= to[s]) {
            if (begins[s] < 0) begins[s] = film[i]
            ends[s] = film[i]
          }
        }
        if (begins[s] < 0) printf "%s drawn at %.2f s has no frame in the film\n", name[s], from[s] / 1000
      }
      w = 0
      for (j = 1; j <= k; j++) {
        for (s = 1; s <= m; s++) {
          if (begins[s] < 0) continue
          inside = (w + lasts[j] < ends[s] ? w + lasts[j] : ends[s]) - (w > begins[s] ? w : begins[s])
          if (inside > limit) {
            printf "frame %d at %.2f s plays %d ms inside %s\n", j, w / 1000, inside, name[s]
            break
          }
        }
        w += lasts[j]
      }
    }' "${root}/tmp/walkthrough/walkthrough.txt" "$1"
}

# brief prints each WebP frame that plays brief_ms or less, which a browser
# holds for 100 ms instead, by where it starts in the WebP.
#   brief <facts>
brief() {
  awk -v limit="${brief_ms}" '
    $1 == "frame" {
      k++
      if ($2 <= limit) printf "frame %d at %.2f s plays %d ms, which a browser plays for 100 ms\n", k, w / 1000, $2
      w += $2
    }' "$1"
}

# check_keys stops the run when the film kept no keyframe, or a keyframe its
# manifest names is not there to look at.
check_keys() {
  local film="${root}/tmp/walkthrough" file caption count=0
  while read -r file caption; do
    [[ -f "${film}/${file}" ]] || fail "the walkthrough's keyframe for \"${caption}\" is missing: ${film}/${file}"
    count=$((count + 1))
  done < <(awk '$1 == "key" { file = $2; $1 = $2 = $3 = ""; sub(/^ +/, ""); print file, $0 }' "${film}/walkthrough.txt")
  ((count > 0)) || fail "the walkthrough kept no keyframe"
  echo "ok    web-walkthrough.webp: ${count} keyframes to look at in ${film}/keys"
}

# check_fidelity stops the run when a frame the WebP holds is not the film's:
# web's media:check (web/e2e/docs/fidelity.spec.ts) decodes it as Chromium
# shows it and compares each frame a beat holds, and the last, with the
# film's own, naming the moment and the frame of any that differs.
check_fidelity() {
  (cd "${root}/web" && corepack yarn media:check) \
    || fail "web-walkthrough.webp: a frame it holds is not the film's; the check above names it"
  echo "ok    web-walkthrough.webp: every frame it holds is the film's"
}

# check_walkthrough stops the run when the WebP is not the film the manifest
# describes: animated, its canvas the film's size, looping forever, with no
# frame a browser holds for 100 ms however briefly it plays, as long as the
# film within a second, with no frame lost inside a stretch that moves
# throughout, and every frame it holds the film's own.
check_walkthrough() {
  local webp="${images}/web-walkthrough.webp" facts="${root}/tmp/walkthrough/facts.txt" held length runs lost
  webp_facts "${webp}" >"${facts}"
  if grep -qx 'not webp\|not animated' "${facts}"; then
    fail "web-walkthrough.webp: not an animated WebP"
  fi
  [[ "$(awk '$1 == "canvas" { print $2, $3 }' "${facts}")" == "$(manifest_value size)" ]] \
    || fail "web-walkthrough.webp: its canvas is not the film's $(manifest_value size)"
  [[ "$(awk '$1 == "loop" { print $2 }' "${facts}")" == 0 ]] || fail "web-walkthrough.webp: does not loop forever"
  held="$(brief "${facts}")"
  [[ -z "${held}" ]] || fail "web-walkthrough.webp: frames a browser plays longer than the film:
${held}"
  length="$(manifest_value length)"
  runs="$(awk '$1 == "frame" { total += $2 } END { printf "%d", total }' "${facts}")"
  if ((runs - length > 1000 || length - runs > 1000)); then
    fail "web-walkthrough.webp: runs $(seconds "${runs}") s where the film's $(seconds "${length}") s do"
  fi
  lost="$(stalls "${facts}")"
  [[ -z "${lost}" ]] || fail "web-walkthrough.webp: frames lost where the film moves:
${lost}"
  echo "ok    web-walkthrough.webp: runs $(seconds "${runs}") s, as the film's $(seconds "${length}") s do"
  check_fidelity
  check_keys
}
