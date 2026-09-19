#!/usr/bin/env bash
# Every .go file starts with the SPDX header CONTRIBUTING.md requires:
#
#   // Copyright 2026 Jacob Delgado
#   // SPDX-License-Identifier: Apache-2.0
#
# With no arguments, checks every tracked .go file; with arguments, only those
# (that is how lefthook passes the staged set).
set -euo pipefail

readonly EXPECTED_COPYRIGHT='// Copyright 2026 Jacob Delgado'
readonly EXPECTED_SPDX='// SPDX-License-Identifier: Apache-2.0'

files=("$@")
if [[ ${#files[@]} -eq 0 ]]; then
  # Prefer git, so ignored and vendored files stay out of the check. Before
  # `git init` — or in an export tarball — fall back to a filesystem walk, since
  # "git told me nothing" and "there are no Go files" must not look alike: that
  # is how this check quietly passes on every file in the repo.
  if git rev-parse --git-dir >/dev/null 2>&1; then
    # --others --exclude-standard includes files that are not committed yet but
    # are not ignored either. Without it, a fresh repo whose files are all
    # untracked reports "no Go files" and passes every one of them.
    while IFS= read -r tracked; do
      files+=("${tracked}")
    done < <(git ls-files --cached --others --exclude-standard '*.go')
  else
    while IFS= read -r found; do
      files+=("${found}")
    done < <(find . -name '*.go' -not -path './bin/*' -not -path './dist/*' -not -path './tmp/*')
  fi

  if [[ ${#files[@]} -eq 0 ]]; then
    echo "License headers: no Go files to check."
    exit 0
  fi
fi

failures=0
for f in "${files[@]}"; do
  [[ -f "${f}" ]] || continue
  # Generated Go carries oapi-codegen's "DO NOT EDIT" banner, not the SPDX
  # header; it is machine-written and excluded, guarded by a drift gate instead.
  [[ "${f}" == *.gen.go ]] && continue
  first="$(sed -n 1p "${f}")"
  second="$(sed -n 2p "${f}")"
  if [[ "${first}" != "${EXPECTED_COPYRIGHT}" ]] || [[ "${second}" != "${EXPECTED_SPDX}" ]]; then
    echo "missing or malformed license header: ${f}" >&2
    failures=$((failures + 1))
  fi
done

if [[ "${failures}" -gt 0 ]]; then
  echo >&2
  echo "Add these two lines to the top of each file listed above:" >&2
  echo "  ${EXPECTED_COPYRIGHT}" >&2
  echo "  ${EXPECTED_SPDX}" >&2
  exit 1
fi

echo "License headers: ${#files[@]} file(s) checked."
