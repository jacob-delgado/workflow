#!/usr/bin/env bash
#
# Tests for check-commit-message.sh: every message below is checked, and the
# script must accept or refuse it as the case says.
#
# Usage:
#   scripts/check-commit-message_test.sh
set -euo pipefail

readonly check="${BASH_SOURCE[0]%/*}/check-commit-message.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# expect runs the check on a message and compares the verdict.
#   expect <accept|refuse> <name> <message>
expect() {
  local want="$1" name="$2" message="$3" got
  local file="${workdir}/message"

  printf '%s\n' "${message}" >"${file}"
  cases=$((cases + 1))

  if "${check}" "${file}" >/dev/null 2>&1; then
    got="accept"
  else
    got="refuse"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
}

expect accept "a plain subject" "feat(jira): add issue transition command"
expect accept "a subject and body" "fix: keep text from servers from driving the terminal

Jira returned escape sequences."
expect refuse "no type" "add issue transition command"
expect refuse "an unknown type" "feature: add issue transition command"
expect refuse "an empty message" ""

# CLAUDE.md: subject ≤ 72 characters, no trailing period.
expect refuse "a subject over 72 characters" "feat: $(printf 'x%.0s' {1..70})"
expect accept "a subject at exactly 72 characters" "feat: $(printf 'x%.0s' {1..66})"
expect refuse "a subject ending in a period" "fix: redact the token."

# Under `git commit -v` the message file carries the diff below a scissors line,
# and git's boilerplate comment lines above it; neither is the commit message,
# so a BREAKING-CHANGE in a diff hunk must not refuse the commit.
expect accept "a diff below the scissors line is ignored" "$(printf 'feat: add a thing\n\nA real body.\n# ------------------------ >8 ------------------------\ndiff --git a/x b/x\n+BREAKING-CHANGE: in the diff, not the message')"
expect accept "a commented example is ignored" "$(printf 'feat: add a thing\n\nReal body.\n# BREAKING-CHANGE: only an example in a comment')"

# The message that made release-please propose 0.1.0 for a docs change: a
# wrapped sentence left "BREAKING CHANGE:" at the start of a line.
expect refuse "a wrapped sentence that reads as a breaking footer" "docs: correct the pre-1.0 version bump rules

CONTRIBUTING.md claimed \"feat bumps the minor version\", which is the
opposite of what release-please is configured to do and what this
project wants. Before 1.0 the minor digit is reserved for breaking
changes: feat and fix both bump the patch, and only feat! or a
BREAKING CHANGE: footer bumps the minor.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
expect refuse "the hyphenated footer without !" "feat: drop the old flag

BREAKING-CHANGE: --old is gone"
expect refuse "the # footer form without !" "feat: drop the old flag

BREAKING CHANGE #12"

expect refuse "! only in the description" "fix: stop crashing!

BREAKING CHANGE: the old flag is gone"

# release-please 17.6.0 marks each of these breaking too, through paths that
# do not start a line with the footer; each was checked against its parser.
expect refuse "the hyphenated footer mid-line" "docs: explain version bumps

Only feat! or a BREAKING-CHANGE: footer bumps the minor."
expect refuse "a footer indented under another trailer" "fix: stop the retry loop

Refs: #12, which explains why this is not a
  BREAKING CHANGE: nothing depended on it"
expect refuse "a footer indented with a no-break space" "$(printf 'fix: a\n\nRefs: x\n\302\240BREAKING CHANGE: b')"
expect refuse "a token!: footer" "docs: tidy wording

See the rules.
Note!: nothing breaks here"
expect refuse "an example subject at the end of the body" "docs: show a breaking subject

For example:

feat(cli)!: rename doctor to check

Co-Authored-By: A <a@example.com>"
expect refuse "a paragraph release-please splits off" "fix: a

feat(x)!: b (c): d

Plain body paragraph."
expect refuse "a nested commit mid-sentence" "fix: a

See BEGIN_NESTED_COMMIT feat!: b END_NESTED_COMMIT here."
expect refuse "a footer after a bare carriage return" "$(printf 'fix: a\r\rBREAKING CHANGE: b')"

expect accept "CRLF line endings" "$(printf 'feat: a\r\n\r\nBody.\r\n')"
expect accept "a trailer with parentheses" "fix: a

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
expect accept "a breaking change marked both ways" "feat!: drop the old flag

BREAKING CHANGE: --old is gone"
expect accept "a scoped breaking change marked both ways" "fix(config)!: reject unknown keys

BREAKING-CHANGE: a misspelled key is now an error"
expect accept "a breaking subject with no footer" "feat(cli)!: rename doctor to check"
expect accept "the phrase mid-line" "docs: explain version bumps

Only feat! or a BREAKING CHANGE: footer bumps the minor."

if ((failures > 0)); then
  echo "check-commit-message: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-commit-message: all ${cases} cases pass."
