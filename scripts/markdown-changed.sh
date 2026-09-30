#!/usr/bin/env bash
# markdown-changed.sh — say whether a change needs its Markdown linted: whether
# anything since <base> touches docs/, any .md file, or the markdownlint config
# that decides what the lint accepts.
#
# Usage: markdown-changed.sh <base-ref>
#
# Exits 0 and names the files when the Markdown needs linting, 1 when the
# change is to other files alone, and 2 when git cannot compare with <base>, so
# a caller that cannot tell lints rather than skips. CI's Lint job asks this of
# a pull request; a push to main lints every Markdown file regardless.
set -uo pipefail

if (($# != 1)); then
  echo "usage: markdown-changed.sh <base-ref>" >&2
  exit 2
fi
readonly base="${1}"

if ! changed="$(git diff --name-only "${base}...HEAD" 2>/dev/null)"; then
  echo "markdown-changed.sh: cannot compare with ${base}" >&2
  exit 2
fi

matching="$(grep -E '^docs/|\.md$|^\.markdownlint-cli2\.yaml$' <<<"${changed}" || true)"
if [[ -z "${matching}" ]]; then
  echo "No Markdown, docs/ or markdownlint config changed since ${base}."
  exit 1
fi

echo "Markdown to lint, changed since ${base}:"
echo "${matching}"
