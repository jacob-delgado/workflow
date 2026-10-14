#!/usr/bin/env bash
# Commit a release's Homebrew formula to the tap.
#
# Usage:
#   publish-packages.sh <tag> <source repository> <packages dir> <tap repository>
#
# Takes <packages dir>/Formula/workflow.rb, as package-manifests.sh writes it
# for <tag>, and commits it to the same path in <tap repository> through
# GitHub's contents API: one commit, on the default branch, with no clone, no
# git identity and no credential file. A formula already as written is left
# alone, so a run repeated for the same release commits nothing.
#
# Only the latest release of <source repository> is published. That one rule
# refuses a prerelease, which is never the latest, and stops a run for an older
# tag from moving the formula back; either is said and exits 0.
#
# Environment:
#   GH_TOKEN  a token that can write the tap's contents, never printed. Without one this warns and exits 0: the release is out
#             by the time this runs, and a red run would read as a failed
#             release. An expired or narrower token fails when it writes.
#
# Exits 0 when the formula names the release, or is left alone as above; 1
# when it is missing or a read or write fails; and 2 for a usage error. Run by
# .github/workflows/packages.yml.
set -euo pipefail

readonly formula="Formula/workflow.rb"
readonly script="scripts/release/publish-packages.sh"

errors="$(mktemp)"
readonly errors
trap 'rm -f "${errors}"' EXIT

# usage says how to run this, and exits 2.
usage() {
  echo "Usage:" >&2
  echo "  ${script} <tag> <source repository> <packages dir> <tap repository>" >&2
  exit 2
}

# fail says why nothing more is published, and exits 1.
#   fail <reason>
fail() {
  echo "$1" >&2
  exit 1
}

# read_api prints what a gh api read answers, or says what GitHub answered
# and exits 1. A not found answer prints nothing when the caller allows it.
#   read_api <allow-missing|require> <endpoint> <jq filter>
read_api() {
  local answer
  if answer="$(gh api "$2" --jq "$3" 2>"${errors}")"; then
    printf '%s\n' "${answer}"
    return
  fi
  if [[ "$1" == allow-missing ]] && grep -q 'HTTP 404' "${errors}"; then
    return
  fi
  cat "${errors}" >&2
  fail "Could not read $2."
}

# publish commits a file to a repository's same path, unless the blob there is
# already the file's.
#   publish <repository> <packages dir> <path> <message> <tag>
publish() {
  local repository="$1" file="$2/$3" path="$3" message="$4" tag="$5" blob remote
  blob="$(git hash-object --no-filters "${file}")"
  remote="$(read_api allow-missing "repos/${repository}/contents/${path}" .sha)"
  if [[ "${remote}" == "${blob}" ]]; then
    echo "${repository}: ${path} already names ${tag}"
    return
  fi

  local -a fields=(-f "message=${message}" -f "content=$(base64 <"${file}" | tr -d '\n')")
  if [[ -n "${remote}" ]]; then
    fields+=(-f "sha=${remote}")
  fi
  if ! gh api --method PUT "repos/${repository}/contents/${path}" "${fields[@]}" >/dev/null 2>"${errors}"; then
    cat "${errors}" >&2
    fail "Could not write ${path} to ${repository}."
  fi
  echo "${repository}: ${path} now names ${tag}"
}

main() {
  if (($# != 4)); then
    usage
  fi
  local tag="$1" source="$2" packages="$3" tap="$4" latest
  if [[ -z "${GH_TOKEN:-}" ]]; then
    echo "::warning::No PACKAGES_TOKEN in the packages environment, so ${tap} is left as it is; .github/workflows/packages.yml says how to set it up"
    exit 0
  fi
  if [[ ! -f "${packages}/${formula}" ]]; then
    fail "${packages}/${formula} was not written; nothing is published."
  fi

  latest="$(read_api require "repos/${source}/releases/latest" .tag_name)"
  if [[ "${latest}" != "${tag}" ]]; then
    echo "${tag} is not the latest release (${latest} is); the formula stays as it is"
    exit 0
  fi

  publish "${tap}" "${packages}" "${formula}" "workflow ${tag#v}" "${tag}"
}

main "$@"
