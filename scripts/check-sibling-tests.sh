#!/usr/bin/env bash
#
# Fail on a script under scripts/ that has no sibling test.
#
# Usage:
#   scripts/check-sibling-tests.sh
#
# scripts/CLAUDE.md: every script here, and in any directory below, has a
# sibling <name>_test.sh that drives it, and a new script brings its test in
# the same commit. This holds that: each tracked script, a file in a language
# a gate is written in (.sh, .bash, .mjs, .js, .awk, .py), wants a tracked
# <name>_test.sh beside it, its name without the extension. A test is no
# script of its own, nor is a data file. The libraries in scripts/lib/ are
# driven through their callers' tests, as that file says, so they want none.
#
#   TRACKED FILES ONLY, like the file-length gate: `git ls-files` measures what
#   is committed or staged, so a test left unadded fails until it is added.
#
# Exits 0 when every script has its test, 1 when one has none, and 2 when git
# cannot list the files or lists no script.
set -euo pipefail

if ! top="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  echo "check-sibling-tests: not inside a git repository, so there is nothing to measure." >&2
  exit 2
fi
cd "${top}"

if ! files="$(git ls-files -- scripts)"; then
  echo "check-sibling-tests: could not list tracked files with git." >&2
  exit 2
fi

scripts="$(grep -E '\.(sh|bash|mjs|js|awk|py)$' <<<"${files}" | grep -vE '_test\.sh$|^scripts/lib/' || true)"
if [[ -z "${scripts}" ]]; then
  echo "check-sibling-tests: git lists no script under scripts/, so there is nothing to measure." >&2
  exit 2
fi

failed=0
count=0
while read -r script; do
  count=$((count + 1))
  sibling="${script%.*}_test.sh"
  if ! grep -qxF -- "${sibling}" <<<"${files}"; then
    echo "${script} has no sibling test ${sibling}" >&2
    failed=1
  fi
done <<<"${scripts}"

if ((failed)); then
  echo "check-sibling-tests: give each script a test beside it that drives it (scripts/CLAUDE.md)." >&2
  exit 1
fi
echo "check-sibling-tests: ${count} script(s) under scripts/, each with its sibling test."
