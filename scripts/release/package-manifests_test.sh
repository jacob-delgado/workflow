#!/usr/bin/env bash
#
# Tests for package-manifests.sh. Each case writes a release's checksums the
# way release.yml does, runs the script on them, and reads what it wrote: the
# Homebrew formula, word for word or against the rules brew style and brew
# audit --strict hold a tap's formula to, or nothing at all when it refuses.
#
# Usage:
#   scripts/release/package-manifests_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/package-manifests.sh"

# shellcheck source=../lib/testing.sh
source "${here}/../lib/testing.sh"

readonly repository="jacob-delgado/workflow"
readonly macos_sum="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
readonly linux_sum="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
readonly windows_sum="cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
readonly other_sum="dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

# The three binaries, as `sha256sum workflow_*` lists them in dist/.
binaries="${macos_sum}  workflow_darwin_arm64
${linux_sum}  workflow_linux_amd64
${windows_sum}  workflow_windows_amd64.exe"
readonly binaries

# What release.yml publishes: it writes each binary's .sha256 first, so its
# SHA256SUMS lists those files' own checksums among the binaries'.
release_sums="${macos_sum}  workflow_darwin_arm64
${other_sum}  workflow_darwin_arm64.sha256
${linux_sum}  workflow_linux_amd64
${other_sum}  workflow_linux_amd64.sha256
${windows_sum}  workflow_windows_amd64.exe
${other_sum}  workflow_windows_amd64.exe.sha256"
readonly release_sums

# sums writes a checksums file for a case and prints its path.
#   sums <text>
sums() {
  local file="${workdir}/sums${cases}"
  : >"${file}"
  if [[ -n "$1" ]]; then
    printf '%s\n' "$1" >"${file}"
  fi
  printf '%s\n' "${file}"
}

# generate runs the script into the case's own out directory, which the case
# reads as ${workdir}/out<case>.
#   generate <tag> <checksums file> [repository]
generate() {
  "${script}" "$1" "$2" "${3:-${repository}}" "${workdir}/out${cases}"
}

# expect_file compares a file the last case wrote with what it should hold, as
# a case of its own.
#   expect_file <name> <path under the out directory> <want>
expect_file() {
  local name="$1" file="${workdir}/out${cases}/$2" want="$3"
  count_case
  if [[ ! -f "${file}" ]]; then
    fail_case "${name}" "$2 was not written"
    return
  fi
  if ! diff -u <(printf '%s\n' "${want}") "${file}" >"${workdir}/diff" 2>&1; then
    fail_case "${name}" "$2 is not what a release should get:"
    sed 's/^/    /' "${workdir}/diff" >&2
  fi
}

# expect_nothing_written wants the last case to have left no out directory, as
# a case of its own.
#   expect_nothing_written <name>
expect_nothing_written() {
  local out="${workdir}/out${cases}"
  count_case
  if [[ -e "${out}" ]]; then
    fail_case "$1" "it wrote $(find "${out}" -type f | sed "s|${workdir}/||" | tr '\n' ' ')"
  fi
}

# refuses wants the script to refuse a tag, checksums and repository, saying
# a text, and to write nothing.
#   refuses <name> <text> <tag> <checksums file> [repository]
refuses() {
  local name="$1" text="$2"
  shift 2
  expect_output 1 "${name}" "${text}" generate "$@"
  expect_nothing_written "${name}"
}

# formula_for prints the formula a release of a tag should get, word for word.
# A prerelease also names its version, which Homebrew cannot scan from its url.
#   formula_for <tag> [version]
formula_for() {
  local download="https://github.com/jacob-delgado/workflow/releases/download/$1" version=""
  if [[ -n "${2:-}" ]]; then
    version="
  version \"$2\""
  fi
  cat <<FORMULA
# Written by scripts/release/package-manifests.sh in jacob-delgado/workflow
# for each release; an edit here is lost at the next one.
class Workflow < Formula
  desc "Run your Jira, Git forge and messaging workflow, in the terminal or a browser"
  homepage "https://github.com/jacob-delgado/workflow"
  if OS.mac?
    url "${download}/workflow_darwin_arm64"
    sha256 "${macos_sum}"
  else
    url "${download}/workflow_linux_amd64"
    sha256 "${linux_sum}"
  end${version}
  license "Apache-2.0"

  on_macos do
    depends_on arch: :arm64
  end

  on_linux do
    depends_on arch: :x86_64
  end

  def install
    bin.install Dir["workflow_*"].first => "workflow"
    chmod 0755, bin/"workflow"
    generate_completions_from_executable(bin/"workflow", "completion")
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/workflow --version")
  end
end
FORMULA
}

formula="$(formula_for v1.4.0)"
readonly formula

# brew_rejects prints each line of a formula that `brew style` or
# `brew audit --strict` rejects in a tap, with the rule it breaks. Homebrew's
# FormulaAudit/ComponentsOrder lets an on_macos or on_linux block hold
# dependencies and the like, never a url or a sha256. The audit refuses a
# version a release's url already gives, as v1.4.0's does; a prerelease's url
# gives none, so formula_for pins its version line instead. And
# FormulaAudit/RedundantGenerateCompletionsShells refuses naming the shells
# Homebrew writes completions for by default, which are bash, zsh and fish.
#   brew_rejects <formula file>
brew_rejects() {
  awk '
    /^  on_(macos|linux) do$/ { block = $1 }
    /^  end$/ { block = "" }
    block != "" && ($1 == "url" || $1 == "sha256") {
      print "ComponentsOrder: " block " cannot include " $1 ": " $0
    }
    /releases\/download\/v[0-9]+\.[0-9]+\.[0-9]+\// { release = 1 }
    $1 == "version" { version = $0 }
    /shells: \[:bash, :zsh, :fish\]/ { print "RedundantGenerateCompletionsShells: " $0 }
    END {
      if (release && version != "") {
        print "audit: the version is redundant with the one scanned from the url: " version
      }
    }
  ' "$1"
}

# expect_brew_accepts wants nothing in the formula the last case wrote that
# brew_rejects, as a case of its own.
#   expect_brew_accepts <name>
expect_brew_accepts() {
  local name="$1" file="${workdir}/out${cases}/Formula/workflow.rb"
  count_case
  if [[ ! -f "${file}" ]]; then
    fail_case "${name}" "Formula/workflow.rb was not written"
    return
  fi
  brew_rejects "${file}" >"${workdir}/rejected"
  if [[ -s "${workdir}/rejected" ]]; then
    fail_case "${name}" "Homebrew would reject it:"
    sed 's/^/    /' "${workdir}/rejected" >&2
  fi
}

expect_exit pass "writes the formula for a release" generate v1.4.0 "$(sums "${release_sums}")"
expect_file "writes the formula for a release" Formula/workflow.rb "${formula}"

expect_exit pass "reads the binaries' checksums alone, as CI writes them" generate v1.4.0 "$(sums "${binaries}")"
expect_file "reads the binaries' checksums alone, as CI writes them" Formula/workflow.rb "${formula}"

expect_exit pass "writes nothing for Windows, which installs from the release" \
  generate v1.4.0 "$(sums "${release_sums}")"
written="$(cd "${workdir}/out${cases}" && find . -type f)"
count_case
if [[ "${written}" != "./Formula/workflow.rb" ]]; then
  fail_case "writes nothing for Windows, which installs from the release" "it wrote $(tr '\n' ' ' <<<"${written}")"
fi

expect_exit pass "a release's formula keeps the rules brew style and brew audit --strict hold" \
  generate v1.4.0 "$(sums "${release_sums}")"
expect_brew_accepts "a release's formula keeps the rules brew style and brew audit --strict hold"

expect_exit pass "a prerelease names its version, which its url does not give" \
  generate v1.4.0-rc.1 "$(sums "${release_sums}")"
expect_file "a prerelease names its version, which its url does not give" Formula/workflow.rb \
  "$(formula_for v1.4.0-rc.1 1.4.0-rc.1)"

expect_exit pass "a prerelease's formula keeps the rules brew style and brew audit --strict hold" \
  generate v1.4.0-rc.1 "$(sums "${release_sums}")"
expect_brew_accepts "a prerelease's formula keeps the rules brew style and brew audit --strict hold"

expect_exit pass "writes into an out directory that does not exist yet" \
  "${script}" v1.4.0 "$(sums "${release_sums}")" "${repository}" "${workdir}/deep/er/out"
count_case
if [[ ! -f "${workdir}/deep/er/out/Formula/workflow.rb" ]]; then
  fail_case "writes into an out directory that does not exist yet" "the formula is not there"
fi

for tag in 1.4.0 v1.4 v1.4.0/nested "v1.4.0 and more" v1.4.0-; do
  refuses "a tag that is not a release is refused: '${tag}'" "not a release tag" "${tag}" "$(sums "${release_sums}")"
done

refuses "a platform with no place is refused" \
  "workflow_linux_arm64 has no place in the Homebrew formula, nor is it named as installed from the release" \
  v1.4.0 "$(sums "${release_sums}
${other_sum}  workflow_linux_arm64")"

refuses "a missing platform is refused" "no checksum for workflow_linux_amd64" \
  v1.4.0 "$(sums "${macos_sum}  workflow_darwin_arm64
${windows_sum}  workflow_windows_amd64.exe")"

refuses "a platform listed twice is refused" "workflow_linux_amd64 is listed twice" \
  v1.4.0 "$(sums "${release_sums}
${other_sum}  workflow_linux_amd64")"

refuses "a hash one digit short is refused" "not a checksum line" \
  v1.4.0 "$(sums "${release_sums}
${other_sum:1}  workflow_linux_arm64")"

refuses "a binary-mode checksum line is refused" "not a checksum line" \
  v1.4.0 "$(sums "${macos_sum} *workflow_darwin_arm64
${linux_sum}  workflow_linux_amd64
${windows_sum}  workflow_windows_amd64.exe")"

refuses "an empty checksums file is refused" "no checksum for workflow_darwin_arm64" \
  v1.4.0 "$(sums "")"

for name in 'owner/na"me' workflow 'owner/name/more' '/name'; do
  refuses "a repository that is not owner/name is refused: '${name}'" "not an owner/name repository" \
    v1.4.0 "$(sums "${release_sums}")" "${name}"
done

refuses "a checksums file that does not exist is refused" "cannot read" v1.4.0 "${workdir}/absent"

expect_output 2 "no arguments is a usage error" "Usage:" "${script}"
expect_output 2 "too few arguments is a usage error" "Usage:" "${script}" v1.4.0 "$(sums "${release_sums}")" "${repository}"
expect_output 2 "too many arguments is a usage error" "Usage:" \
  "${script}" v1.4.0 "$(sums "${release_sums}")" "${repository}" "${workdir}/out" extra

finish_tests
