#!/usr/bin/env bash
# docs-only.sh — say whether a change is to the documentation alone, so CI and
# gate-as-dev can check it with the docs gates (task check:docs) and leave the
# Go, web and end-to-end suites out: whether every file changed since <base> is
# Markdown outside web/, the Hugo site's configuration and module files under
# docs/, or the configuration of the two linters that read only prose,
# .markdownlint-cli2.yaml and _typos.toml.
#
# The list names what is docs and nothing else, so a path it does not know
# counts as code and gets every check: an image or a script under docs/ among
# them. Markdown under web/ is code, since the web's prettier check reads it. A
# deleted file and both ends of a rename count by their own paths, so a code
# file deleted or renamed to a Markdown name is a code change.
#
# Usage: docs-only.sh <base-ref>
#
# Exits 0 and names the files when the change is docs only, 1 when it changes
# anything else or nothing at all, and 2 when git cannot compare with <base>, so
# a caller that cannot tell runs every check rather than too few.
set -uo pipefail

if (($# != 1)); then
  echo "usage: docs-only.sh <base-ref>" >&2
  exit 2
fi
readonly base="${1}"

if ! changed="$(git diff --name-only --no-renames "${base}...HEAD" 2>/dev/null)"; then
  echo "docs-only.sh: cannot compare with ${base}" >&2
  exit 2
fi

if [[ -z "${changed}" ]]; then
  echo "Nothing changed since ${base}, so nothing is known to be docs only."
  exit 1
fi

readonly docs='\.md$|^docs/.*\.toml$|^docs/go\.(mod|sum)$|^\.markdownlint-cli2\.yaml$|^_typos\.toml$'
code="$(
  grep -E '^web/' <<<"${changed}"
  grep -v -E "${docs}" <<<"${changed}" | grep -v -E '^web/'
)"
if [[ -n "${code}" ]]; then
  echo "Not docs only: these changed since ${base}:"
  echo "${code}"
  exit 1
fi

echo "Docs only, changed since ${base}:"
echo "${changed}"
