#!/usr/bin/env bash
#
# Fail if a generated tree differs from what git holds.
#
# Usage:
#   scripts/check-generated.sh <path> <what to do about it>
#
# Run right after the generator, from the directory the path is relative to.
# A tracked file the generator changed fails, and so does a file it added that
# nobody committed: `git diff` alone misses a new file, which is what a
# generator upgrade that emits one more file leaves behind. Ignored files do
# not count. A path git tracks nothing under fails too, so a mistyped path
# cannot pass on nothing compared.
#
# gen:verify, the web's gen:check and web:dist:check all run this, so the three
# drift checks agree on what drift is.
set -euo pipefail

if (($# != 2)); then
  sed -n '5,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi
readonly path="$1" advice="$2"

if ! git ls-files --error-unmatch -- "${path}" >/dev/null 2>&1; then
  echo "check-generated: git tracks nothing under ${path}" >&2
  exit 1
fi

if ! git diff --quiet -- "${path}" \
  || [[ -n "$(git ls-files --others --exclude-standard -- "${path}")" ]]; then
  echo "${advice}" >&2
  git status --short --untracked-files=all -- "${path}" >&2
  exit 1
fi
