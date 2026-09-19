#!/usr/bin/env bash
#
# Fail if the Go version drifts between the files that pin it.
#
# Usage:
#   scripts/check-go-version.sh          # the gate
#   scripts/check-go-version.sh --list   # each source and the version read
#
# Go's version is written in five places — go.mod, docs/go.mod, mise.toml,
# build/Dockerfile and the README badge — and no single source feeds the rest. A
# bump that updates some but not all leaves the build one Go and the container
# another, which the "everything agrees" audit note only ever caught by hand.
# This is the mechanical half: the MAJOR.MINOR series must match everywhere, and
# the files that pin a full X.Y.Z toolchain must pin the same one. go.mod and the
# badge legitimately carry only the series, so they are not held to a patch.
#
# Paths are read from the current directory, so `task` runs it from the repo
# root and its own test runs it against a fixture tree.
set -euo pipefail

fail() {
  echo "check-go-version: $*" >&2
  exit 1
}

# read_version pulls one version out of a file with a sed program, and fails
# loudly rather than letting an empty match pass as agreement.
read_version() {
  local label="$1" file="$2" program="$3" value
  [[ -f "${file}" ]] || fail "no ${label} at ${file}"
  value="$(sed -n "${program}" "${file}" | head -1)"
  [[ -n "${value}" ]] || fail "could not read the Go version from ${label} (${file})"
  echo "${value}"
}

# series is a version's MAJOR.MINOR, so 1.27.1 and 1.27 share the series 1.27.
series() {
  echo "$1" | cut -d. -f1,2
}

names=("go.mod" "docs/go.mod" "mise.toml" "build/Dockerfile" "README.md")
values=(
  "$(read_version "go.mod" "go.mod" 's/^go \([0-9][0-9.]*\).*/\1/p')"
  "$(read_version "docs/go.mod" "docs/go.mod" 's/^go \([0-9][0-9.]*\).*/\1/p')"
  "$(read_version "mise.toml" "mise.toml" 's/^go = "\([0-9][0-9.]*\)".*/\1/p')"
  "$(read_version "build/Dockerfile" "build/Dockerfile" 's/^ARG GO_VERSION=\([0-9][0-9.]*\).*/\1/p')"
  "$(read_version "README.md" "README.md" 's|.*/badge/go-\([0-9][0-9.]*\)-.*|\1|p')"
)

if [[ "${1:-}" == "--list" ]]; then
  for i in "${!names[@]}"; do
    printf '%-18s %s\n' "${names[${i}]}" "${values[${i}]}"
  done
  exit 0
fi

if [[ -n "${1:-}" ]]; then
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

want_series="$(series "${values[0]}")"
want_full=""
drift=0

for i in "${!names[@]}"; do
  value="${values[${i}]}"
  value_series="$(series "${value}")"

  if [[ "${value_series}" != "${want_series}" ]]; then
    drift=1
  fi

  # A three-part version pins a toolchain; those must all be the same one.
  if [[ "${value}" == *.*.* ]]; then
    if [[ -z "${want_full}" ]]; then
      want_full="${value}"
    elif [[ "${value}" != "${want_full}" ]]; then
      drift=1
    fi
  fi
done

if ((drift > 0)); then
  echo "check-go-version: the Go version disagrees across its sources:" >&2
  for i in "${!names[@]}"; do
    printf '  %-18s %s\n' "${names[${i}]}" "${values[${i}]}" >&2
  done
  echo "Bring the series (and any pinned patch) into step." >&2
  exit 1
fi

echo "check-go-version: every source pins Go ${want_series} (toolchain ${want_full:-unpinned})."
