#!/usr/bin/env bash
#
# Tests for check-tool-checksums.sh: it passes when build/tool-checksums.txt
# pins a SHA-256 for each tool the build container downloads, at mise.toml's
# version, for both architectures, and nothing else, and when every download
# in build/Dockerfile is verified. It fails, naming the gap, on a version bump
# without its checksums, an architecture left out, a line for a version no
# longer pinned, a hash that is not one, and a download with no verification.
#
# Usage:
#   scripts/check-tool-checksums_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-tool-checksums.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly hash="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
readonly tools="node shellcheck jq lefthook typos hadolint taplo zizmor"

# tree writes mise.toml, a Dockerfile with one verified download, and a
# checksums file pinning every tool at version 1.0.0 into a fresh directory.
#   tree <dir>
tree() {
  local dir="$1" tool arch
  mkdir -p "${dir}/build"
  : >"${dir}/mise.toml"
  printf '# pins\n' >"${dir}/build/tool-checksums.txt"
  for tool in ${tools}; do
    printf '%s = "1.0.0"\n' "${tool}" >>"${dir}/mise.toml"
    for arch in amd64 arm64; do
      printf '%s 1.0.0 %s %s\n' "${tool}" "${arch}" "${hash}" >>"${dir}/build/tool-checksums.txt"
    done
  done
  cat >"${dir}/build/Dockerfile" <<'EOF'
RUN set -eux; \
    curl -fsSL "https://example.com/node.tar.xz" -o /tmp/node.tar.xz; \
    verify-download /usr/local/share/tool-checksums.txt /tmp/node.tar.xz node "${NODE_VERSION}" "${arch}"
EOF
}

# bump sets one tool's pin in a tree's mise.toml.
#   bump <dir> <tool> <version>
bump() {
  sed -i.bak "s/^$2 = .*/$2 = \"$3\"/" "$1/mise.toml"
}

# drop deletes the checksums lines matching a pattern.
#   drop <dir> <pattern>
drop() {
  sed -i.bak "/$2/d" "$1/build/tool-checksums.txt"
}

agree="${workdir}/agree"
tree "${agree}"
expect_exit pass "every download pinned at its version" run_in "${agree}" "${check}"

bumped="${workdir}/bumped"
tree "${bumped}"
bump "${bumped}" node 1.1.0
expect_output fail "a version bumped without its checksums" "no SHA-256 for node 1.1.0 amd64" \
  run_in "${bumped}" "${check}"

stale="${workdir}/stale"
tree "${stale}"
bump "${stale}" jq 1.1.0
printf 'jq 1.1.0 amd64 %s\njq 1.1.0 arm64 %s\n' "${hash}" "${hash}" >>"${stale}/build/tool-checksums.txt"
expect_output fail "a line for a version no longer pinned" "jq 1.0.0 amd64 is not mise.toml's version" \
  run_in "${stale}" "${check}"

one_arch="${workdir}/one-arch"
tree "${one_arch}"
drop "${one_arch}" '^taplo 1.0.0 arm64'
expect_output fail "an architecture left out" "no SHA-256 for taplo 1.0.0 arm64" \
  run_in "${one_arch}" "${check}"

not_hash="${workdir}/not-hash"
tree "${not_hash}"
printf 'zizmor 1.0.0 amd64 xyz\n' >"${not_hash}/build/tool-checksums.txt.new"
drop "${not_hash}" '^zizmor 1.0.0 amd64'
cat "${not_hash}/build/tool-checksums.txt.new" >>"${not_hash}/build/tool-checksums.txt"
expect_output fail "a hash that is not a SHA-256" "zizmor 1.0.0 amd64" run_in "${not_hash}" "${check}"

unverified="${workdir}/unverified"
tree "${unverified}"
printf 'RUN curl -fsSL "https://example.com/jq" -o /usr/local/bin/jq\n' >>"${unverified}/build/Dockerfile"
expect_output fail "a download with no verification" "2 downloads but 1 verification" \
  run_in "${unverified}" "${check}"

missing="${workdir}/missing"
tree "${missing}"
rm "${missing}/build/tool-checksums.txt"
expect_exit fail "no checksums file" run_in "${missing}" "${check}"

finish_tests
