#!/usr/bin/env bash
#
# Print the requirements file that pins yamllint and its whole tree, for
# build/yamllint/requirements.txt.
#
# Usage:
#   scripts/pin-yamllint.sh <yamllint-version> [<package>==<version>...]
#
# `task deps:yamllint` runs this, and `task tools` installs what it wrote.
# yamllint is pinned at the version given: its version decides what the lint
# flags, so it moves only when asked. Each package it needs at run time, and
# each one those need, is pinned at its newest final release that is not
# yanked and whose every file is at least seven days old (CLAUDE.md's age
# gate), unless a <package>==<version> names the one to take: a fix for a known
# vulnerability younger than the week, which
# scripts/dependency-age-exceptions.txt then lists with its advisory. A
# requirement an extra alone needs is left out, and the versions a requirement
# allows are not read: pip refuses a tree that breaks one when `task tools`
# installs it.
#
# Each release is pinned by the sha256 of every wheel PyPI holds for it, every
# interpreter's and platform's, and of no sdist, under --require-hashes and
# --only-binary :all:, so pip installs exactly these files wherever the file
# is used and builds none. It asks PyPI with curl and reads its answers with
# jq alone.
#
# A requirement with an environment marker is refused rather than pinned for
# every platform and interpreter. The file is printed once the whole tree is
# pinned, so a run that stops prints none of it.
#
# Exits 0 having printed the file; 1 when PyPI cannot be read, a package has no
# release old enough, a release pinned has no wheel, a requirement has an
# environment marker, or a <package>==<version> names yamllint or a package
# the tree does not hold, which a misspelled name would otherwise leave at its
# newest release a week old; and 2 when no yamllint version is given or a
# package is named with no version or named twice.
set -euo pipefail

usage() {
  sed -n '7,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
}

# normalized prints each name it reads as PyPI compares names: in lower case,
# each run of -, _ and . one -.
normalized() {
  tr '[:upper:]' '[:lower:]' | sed -E 's/[-_.]+/-/g'
}

if (($# < 1)); then
  usage
fi
declare -A named_once=()
for named in "${@:2}"; do
  if [[ "${named}" != ?*==?* ]]; then
    usage
  fi
  name="$(normalized <<<"${named%%==*}")"
  if [[ -n "${named_once[${name}]:-}" ]]; then
    echo "pin-yamllint: ${name} is named twice" >&2
    exit 2
  fi
  named_once[${name}]=1
done

readonly yamllint_version="$1"
readonly pypi="https://pypi.org/pypi"
readonly min_seconds=$((7 * 24 * 60 * 60))
readonly -a get=(curl -fsSL --retry 2)
now="${PIN_YAMLLINT_NOW:-$(date -u +%s)}"
readonly now
readonly -a named_versions=("${@:2}")

# fetch prints a PyPI JSON document, or stops the run naming the one it could
# not read.
#   fetch <package>[/<version>]
fetch() {
  if ! "${get[@]}" "${pypi}/$1/json"; then
    echo "pin-yamllint: cannot read ${pypi}/$1/json" >&2
    exit 1
  fi
}

# named_version prints the version a <package>==<version> argument names for a
# package, or nothing.
#   named_version <package>
named_version() {
  local named name
  for named in ${named_versions[@]+"${named_versions[@]}"}; do
    name="$(normalized <<<"${named%%==*}")"
    if [[ "${name}" == "$1" ]]; then
      printf '%s' "${named#*==}"
    fi
  done
}

# newest_aged prints a package's newest final release that is not yanked and
# whose every file is at least a week old.
#   newest_aged <package>
newest_aged() {
  local releases version
  releases="$(fetch "$1")"
  version="$(jq -r --argjson cutoff "$((now - min_seconds))" '
    [.releases | to_entries[]
      | select(.key | test("^[0-9]+(\\.[0-9]+)*$"))
      | select((.value | length > 0) and (any(.value[]; .yanked) | not))
      | select(all(.value[]; .upload_time_iso_8601 | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601 <= $cutoff))
      | .key]
    | sort_by(split(".") | map(tonumber)) | last // empty' <<<"${releases}")"
  if [[ -z "${version}" ]]; then
    echo "pin-yamllint: $1 has no final release a week old that is not yanked" >&2
    exit 1
  fi
  printf '%s' "${version}"
}

# version_of prints the version a package is pinned at.
#   version_of <package>
version_of() {
  local named
  named="$(named_version "$1")"
  if [[ "$1" == yamllint ]]; then
    printf '%s' "${yamllint_version}"
  elif [[ -n "${named}" ]]; then
    printf '%s' "${named}"
  else
    newest_aged "$1"
  fi
}

# requirements prints the name of each package a release needs outside any
# extra, as PyPI compares names, or stops the run at one with an environment
# marker: a package only some platforms or interpreters need would be pinned
# for all of them, and so would each package below it.
#   requirements <package> <version> <release-json>
requirements() {
  local marked
  marked="$(jq -r '[.info.requires_dist // [] | .[] | select(test("extra *==") | not) | select(test(";"))]
    | first // empty' <<<"$3")"
  if [[ -n "${marked}" ]]; then
    echo "pin-yamllint: $1 $2 requires '${marked}', and a marker is not pinned: pin it by hand or teach this script markers" >&2
    exit 1
  fi
  jq -r '.info.requires_dist // [] | .[] | select(test("extra *==") | not)
    | capture("^ *(?<name>[A-Za-z0-9][A-Za-z0-9._-]*)").name' <<<"$3" | normalized | sort -u
}

# pin prints a release's requirement, with the sha256 of each of its wheels.
#   pin <package> <version> <release-json>
pin() {
  local wheels sha256
  wheels="$(jq -r '[.urls[] | select(.packagetype == "bdist_wheel") | .digests.sha256] | sort | .[]' <<<"$3")"
  if [[ -z "${wheels}" ]]; then
    echo "pin-yamllint: $1 $2 has no wheel on PyPI, and --only-binary :all: installs nothing else" >&2
    exit 1
  fi
  printf '%s==%s' "$1" "$2"
  while read -r sha256; do
    printf ' \\\n    --hash=sha256:%s' "${sha256}"
  done <<<"${wheels}"
  printf '\n'
}

pins=""
pending=(yamllint)
seen=" yamllint "
next=0
while ((next < ${#pending[@]})); do
  package="${pending[next]}"
  next=$((next + 1))
  version="$(version_of "${package}")"
  release="$(fetch "${package}/${version}")"
  pins+="$(pin "${package}" "${version}" "${release}")"$'\n'
  dependencies="$(requirements "${package}" "${version}" "${release}")"
  while read -r dependency; do
    if [[ -n "${dependency}" && "${seen}" != *" ${dependency} "* ]]; then
      seen="${seen}${dependency} "
      pending+=("${dependency}")
    fi
  done <<<"${dependencies}"
done

# Every package a <package>==<version> names must be one the tree holds, and
# not yamllint, whose version is the first argument.
for named in ${named_versions[@]+"${named_versions[@]}"}; do
  name="$(normalized <<<"${named%%==*}")"
  if [[ "${name}" == yamllint ]]; then
    echo "pin-yamllint: ${named} is named, but yamllint's version is the first argument, ${yamllint_version}" >&2
    exit 1
  elif [[ "${seen}" != *" ${name} "* ]]; then
    echo "pin-yamllint: ${named%%==*} is named, but yamllint ${yamllint_version} does not need it" >&2
    exit 1
  fi
done

cat <<'HEADER'
# yamllint and every package it needs at run time, each pinned by its version
# and by the sha256 of every wheel PyPI holds for it. `task tools` installs
# exactly these, and the age gate dates each file a change adds. Written by
# scripts/pin-yamllint.sh: run `task deps:yamllint` rather than edit it.
--require-hashes
--only-binary :all:
HEADER
printf '%s' "${pins}"
