# shellcheck shell=bash
#
# Reading an animated WebP's own facts, for scripts/docs-media.sh to hold the
# walkthrough to: ffprobe 6.1 cannot read an animated WebP, so this walks its
# RIFF chunks with od. Sourced, it defines webp_facts.

# webp_bytes prints <count> bytes of <file> from <offset> as unsigned
# decimals, one line, space-separated.
#   webp_bytes <file> <offset> <count>
webp_bytes() {
  od -An -v -tu1 -j "$2" -N "$3" "$1" | tr -s ' \n' ' ' | sed 's/^ //; s/ $//'
}

# webp_le prints the little-endian number the decimal bytes given spell.
#   webp_le <byte>...
webp_le() {
  local value=0 shift_by=0 byte
  for byte in "$@"; do
    value=$((value + (byte << shift_by)))
    shift_by=$((shift_by + 8))
  done
  printf '%d' "${value}"
}

# webp_facts prints what an animated WebP says of itself, a line each:
# "canvas <width> <height>", "loop <count>" (0 for forever), then
# "frame <milliseconds>" for each frame, in order. It prints "not animated"
# alone for a file that is a WebP with no ANIM chunk, and "not webp" alone
# for one that is no WebP at all.
#   webp_facts <file>
webp_facts() {
  local file="$1" size offset=12 fourcc length
  local -a head bytes
  read -r -a head <<<"$(webp_bytes "${file}" 0 12)"
  if [[ "$(printf '%b' "$(printf '\\%03o' "${head[@]:0:4}" "${head[@]:8:4}")")" != RIFFWEBP ]]; then
    echo 'not webp'
    return
  fi
  size="$(wc -c <"${file}")"
  local animated=0
  while ((offset + 8 <= size)); do
    read -r -a bytes <<<"$(webp_bytes "${file}" "${offset}" 24)"
    fourcc="$(printf '%b' "$(printf '\\%03o' "${bytes[@]:0:4}")")"
    length="$(webp_le "${bytes[@]:4:4}")"
    case "${fourcc}" in
      VP8X)
        echo "canvas $(($(webp_le "${bytes[@]:12:3}") + 1)) $(($(webp_le "${bytes[@]:15:3}") + 1))"
        ;;
      ANIM)
        animated=1
        echo "loop $(webp_le "${bytes[@]:12:2}")"
        ;;
      ANMF)
        echo "frame $(webp_le "${bytes[@]:20:3}")"
        ;;
      *) ;;
    esac
    offset=$((offset + 8 + length + length % 2))
  done
  if ((! animated)); then
    echo 'not animated'
  fi
}
