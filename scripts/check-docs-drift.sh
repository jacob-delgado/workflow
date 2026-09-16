#!/usr/bin/env bash
# Fail when the committed command reference disagrees with the command tree.
#
# The reference pages under docs/content/docs/reference/ are generated from the
# Cobra commands. Generated files that are also committed have one failure mode —
# somebody adds a flag, never regenerates, and the published documentation
# quietly describes a binary that no longer exists. This is the check that makes
# that impossible rather than merely discouraged.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root
readonly committed="${repo_root}/docs/content/docs/reference"

cd "${repo_root}"

scratch="$(mktemp -d)"
trap 'rm -rf "${scratch}"' EXIT

# _index.md is hand-written and lives alongside the generated pages; copy it in
# so the comparison does not report it as missing.
cp "${committed}/_index.md" "${scratch}/_index.md"

go run ./cmd/docsgen "${scratch}"

if diff -r -u "${committed}" "${scratch}"; then
  echo "Command reference is current."
  exit 0
fi

echo >&2
echo "The command reference is out of date with the command tree." >&2
echo "Regenerate it and commit the result:" >&2
echo >&2
echo "    task docs:gen" >&2
exit 1
