#!/usr/bin/env bash
# Write the Homebrew formula for a release.
#
# Usage:
#   package-manifests.sh <tag> <checksums> <repository> <out dir>
#
# Writes <out dir>/Formula/workflow.rb, for a Homebrew tap, pointing at the
# release <tag> of <repository> (owner/name) on GitHub and pinning the
# checksums the release publishes. <checksums> is that release's SHA256SUMS,
# which release.yml writes from `sha256sum` over every binary
# RELEASE_PLATFORMS in Taskfile.yml builds, and which also lists each binary's
# own .sha256 file; those lines have no package to go in and are passed over.
#
# Every platform a release builds must have its place here: in the formula,
# or named as one installed from the release alone, as Windows is. A binary
# with neither is refused rather than left out unseen, which CI's
# Cross-compile job checks against the binaries every change builds. A tag,
# repository or checksum line of any other shape is refused too, so the values
# written into Ruby below never need escaping.
#
# Exits 0 when the formula is written, 1 when it refuses, and 2 for a usage
# error. Run by .github/workflows/packages.yml for each release, and by
# ci.yml's Cross-compile job, as v0.0.0, for every change.
set -euo pipefail

readonly macos_binary="workflow_darwin_arm64"
readonly linux_binary="workflow_linux_amd64"
# windows_binary is in no package: Windows installs the release's binary
# itself, as install.md's "From a release" says.
readonly windows_binary="workflow_windows_amd64.exe"
readonly description="Run your Jira, Git forge and messaging workflow, in the terminal or a browser"
readonly script="scripts/release/package-manifests.sh"

# usage says how to run this, and exits 2.
usage() {
  echo "Usage:" >&2
  echo "  ${script} <tag> <checksums> <repository> <out dir>" >&2
  exit 2
}

# fail says why the release is refused, and exits 1.
#   fail <reason>
fail() {
  echo "$1" >&2
  exit 1
}

# check_tag refuses a tag that is not a release's, the grammar
# push-release-tag.sh tags with.
#   check_tag <tag>
check_tag() {
  if [[ ! "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
    fail "'$1' is not a release tag, such as v1.4.0 or v1.4.0-rc.1"
  fi
}

# check_repository refuses anything but a GitHub owner/name.
#   check_repository <repository>
check_repository() {
  if [[ ! "$1" =~ ^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$ ]]; then
    fail "'$1' is not an owner/name repository on GitHub"
  fi
}

# check_name refuses a binary that has no place in the formula and is not
# named as installed from the release alone.
#   check_name <binary>
check_name() {
  case "$1" in
    "${macos_binary}" | "${linux_binary}" | "${windows_binary}") ;;
    *) fail "$1 has no place in the Homebrew formula, nor is it named as installed from the release; give it one in ${script}" ;;
  esac
}

# binary_checksums prints the binaries' lines of a checksums file, refusing
# a line of any shape `sha256sum workflow_*` would not write, a binary with no
# place here, and one listed twice.
#   binary_checksums <checksums>
binary_checksums() {
  local line name seen=" "
  while IFS= read -r line || [[ -n "${line}" ]]; do
    if [[ ! "${line}" =~ ^[0-9a-f]{64}\ \ (workflow_[a-z0-9]+_[a-z0-9]+(\.exe)?)(\.sha256)?$ ]]; then
      fail "'${line}' is not a checksum line, '<sha256>  workflow_<os>_<arch>'"
    fi
    name="${BASH_REMATCH[1]}"
    if [[ -n "${BASH_REMATCH[3]}" ]]; then
      continue
    fi
    check_name "${name}"
    if [[ "${seen}" == *" ${name} "* ]]; then
      fail "${name} is listed twice in the checksums"
    fi
    seen+="${name} "
    printf '%s\n' "${line}"
  done <"$1"
}

# checksum_of prints a binary's checksum, refusing one the release lacks.
#   checksum_of <binary checksums> <binary>
checksum_of() {
  local sum
  sum="$(awk -v name="$2" '$2 == name { print $1 }' <<<"$1")"
  if [[ -z "${sum}" ]]; then
    fail "no checksum for $2 in the release's checksums"
  fi
  printf '%s\n' "${sum}"
}

# version_stanza prints the version line a prerelease's formula needs, after
# the newline that sets it off, and nothing for a release. Homebrew scans a
# release's version from its url, so `brew audit --strict` refuses naming it
# again, but reads a prerelease's url as version 64, from the arch.
#   version_stanza <tag>
version_stanza() {
  if [[ "$1" == *-* ]]; then
    printf '\n  version "%s"' "${1#v}"
  fi
}

# write_formula writes the Homebrew formula: the release binary for macOS on
# Apple Silicon and for Linux on x86-64, each refused by name on any other
# machine through its arch requirement, rather than failing on a missing url.
# The url is chosen with OS.mac?, since Homebrew's FormulaAudit/ComponentsOrder
# allows no url in an on_macos or on_linux block, and its
# FormulaAudit/OnSystemConditionals leaves OS.mac? to a tap for this.
# A bare binary is staged without its executable bit, so it gets one before
# Homebrew runs it to write the completions for its default shells, bash, zsh
# and fish.
#   write_formula <file> <tag> <repository> <macOS checksum> <Linux checksum>
write_formula() {
  local download="https://github.com/$3/releases/download/$2" version
  version="$(version_stanza "$2")"
  cat >"$1" <<FORMULA
# Written by ${script} in $3
# for each release; an edit here is lost at the next one.
class Workflow < Formula
  desc "${description}"
  homepage "https://github.com/$3"
  if OS.mac?
    url "${download}/${macos_binary}"
    sha256 "$4"
  else
    url "${download}/${linux_binary}"
    sha256 "$5"
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

main() {
  if (($# != 4)); then
    usage
  fi
  local tag="$1" checksums="$2" repository="$3" out="$4" sums
  check_tag "${tag}"
  check_repository "${repository}"
  if [[ ! -f "${checksums}" || ! -r "${checksums}" ]]; then
    fail "cannot read the checksums ${checksums}"
  fi

  sums="$(binary_checksums "${checksums}")"
  local macos linux
  macos="$(checksum_of "${sums}" "${macos_binary}")"
  linux="$(checksum_of "${sums}" "${linux_binary}")"

  mkdir -p "${out}/Formula"
  write_formula "${out}/Formula/workflow.rb" "${tag}" "${repository}" "${macos}" "${linux}"
  echo "Wrote ${out}/Formula/workflow.rb for ${tag}"
}

main "$@"
