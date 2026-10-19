#!/usr/bin/env bash
#
# Tests for check-media-checksums.sh: it passes when
# docs/media/tool-checksums.txt pins one SHA-256 for ttyd at mise.toml's
# version for each machine scripts/docs-media.sh downloads it for, and
# nothing else. It fails, naming the gap, on a ttyd bump that leaves the old
# lines, a machine left out, a line pinned twice, a hash that is not one, a
# line with a field too many, a line for another tool or machine, a mise.toml
# that pins no ttyd, and a docs-media.sh whose machines it cannot read.
#
# Usage:
#   scripts/check-media-checksums_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-media-checksums.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly hash="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# tree writes a repository whose mise.toml pins ttyd 1.7.7 as an inline
# table, whose docs-media.sh downloads it for x86_64 and aarch64, and whose
# checksums pin it once for each.
#   tree <name>
tree() {
  local dir="${workdir}/$1"
  mkdir -p "${dir}/docs/media" "${dir}/scripts"
  printf '[tools]\nvhs = "0.12.1"\nttyd = { version = "1.7.7", os = ["linux"] }\n' >"${dir}/mise.toml"
  # shellcheck disable=SC2016 # the stand-in script's own text
  printf '  if [[ "$(uname -s)" != Linux || ! "${machine}" =~ ^(x86_64|aarch64)$ ]]; then\n' \
    >"${dir}/scripts/docs-media.sh"
  printf '# pins\nttyd 1.7.7 x86_64 %s\nttyd 1.7.7 aarch64 %s\n' "${hash}" "${hash}" \
    >"${dir}/docs/media/tool-checksums.txt"
  printf '%s' "${dir}"
}

# gate runs the check over a tree.
#   gate <dir>
gate() {
  MEDIA_CHECKSUMS_ROOT="$1" "${check}"
}

dir="$(tree agree)"
expect_output pass "ttyd pinned at mise.toml's version for every machine" "pinned at mise.toml's version" gate "${dir}"

dir="$(tree bumped)"
sed -i.bak 's/1\.7\.7/1.7.8/' "${dir}/mise.toml"
expect_output 1 "a ttyd bump with no lines of its own" "no SHA-256 for ttyd 1.7.8 x86_64" gate "${dir}"
expect_output 1 "a ttyd bump that leaves the old lines" "ttyd 1.7.7 aarch64 is not mise.toml's version of ttyd" \
  gate "${dir}"

dir="$(tree no-arm)"
sed -i.bak '/aarch64/d' "${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a machine left out" "no SHA-256 for ttyd 1.7.7 aarch64" gate "${dir}"

dir="$(tree twice)"
printf 'ttyd 1.7.7 x86_64 %s\n' "${hash}" >>"${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a line pinned twice" "ttyd 1.7.7 x86_64 is pinned 2 times" gate "${dir}"

dir="$(tree short-hash)"
sed -i.bak 's/^\(ttyd 1.7.7 x86_64\) .*/\1 0123abc/' "${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a hash that is not one" "ttyd 1.7.7 x86_64 is pinned to '0123abc', not a SHA-256" gate "${dir}"
dir="$(tree upper-hash)"
sed -i.bak "s/^\(ttyd 1.7.7 x86_64\) .*/\1 $(tr 'a-f' 'A-F' <<<"${hash}")/" "${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a hash in capitals" "not a SHA-256" gate "${dir}"

dir="$(tree extra-field)"
sed -i.bak "s/^\(ttyd 1.7.7 aarch64 .*\)/\1 spare/" "${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a line with a field too many" "${hash} spare', not a SHA-256" gate "${dir}"

dir="$(tree other-tool)"
printf 'vhs 0.12.1 x86_64 %s\n' "${hash}" >>"${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a line for another tool" "vhs 0.12.1 x86_64 is not mise.toml's version of ttyd" gate "${dir}"
dir="$(tree other-machine)"
printf 'ttyd 1.7.7 armhf %s\n' "${hash}" >>"${dir}/docs/media/tool-checksums.txt"
expect_output 1 "a line for a machine docs-media.sh does not download for" \
  "ttyd 1.7.7 armhf is not mise.toml's version of ttyd" gate "${dir}"

dir="$(tree more-machines)"
sed -i.bak 's/x86_64|aarch64/x86_64|aarch64|armhf/' "${dir}/scripts/docs-media.sh"
expect_output 1 "a machine docs-media.sh comes to download for" "no SHA-256 for ttyd 1.7.7 armhf" gate "${dir}"
dir="$(tree no-machines)"
: >"${dir}/scripts/docs-media.sh"
expect_output 1 "a docs-media.sh whose machines cannot be read" "cannot read the machines" gate "${dir}"

dir="$(tree unpinned)"
printf '[tools]\nvhs = "0.12.1"\n' >"${dir}/mise.toml"
expect_output 1 "a mise.toml that pins no ttyd" "mise.toml pins no version of ttyd" gate "${dir}"
dir="$(tree no-checksums)"
rm "${dir}/docs/media/tool-checksums.txt"
expect_output 1 "no checksums file" "cannot read" gate "${dir}"

expect_output 2 "an argument" "Usage:" "${check}" extra

finish_tests
