#!/usr/bin/env bash
#
# Tests for check-commit-message.sh: every message below is checked, and the
# script must accept or refuse it as the case says.
#
# Usage:
#   scripts/check-commit-message_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-commit-message.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# expect checks a message, and wants it accepted (pass) or refused (fail).
#   expect <pass|fail> <name> <message>
expect() {
  local file="${workdir}/message"
  printf '%s\n' "$3" >"${file}"
  expect_exit "$1" "$2" "${check}" "${file}"
}

expect pass "a plain subject" "feat(jira): add issue transition command"
expect pass "a subject and body" "fix: keep text from servers from driving the terminal

Jira returned escape sequences."
expect fail "no type" "add issue transition command"
expect fail "an unknown type" "feature: add issue transition command"
expect fail "an empty message" ""

# CLAUDE.md: subject ≤ 72 characters, no trailing period.
expect fail "a subject over 72 characters" "feat: $(printf 'x%.0s' {1..70})"
expect pass "a subject at exactly 72 characters" "feat: $(printf 'x%.0s' {1..66})"
expect fail "a subject ending in a period" "fix: redact the token."

# CLAUDE.md: the body is wrapped at 72. Prose past it is refused; a URL, the
# trailers, and an indented or fenced block cannot wrap and are left alone.
# Characters are counted, not bytes, so an em dash is one.
prose90="$(printf 'word %.0s' {1..18})"
prose72="$(printf 'x%.0s' {1..72})"
dashes72="$(printf '\342\200\224%.0s' {1..72})"
readonly prose90 prose72 dashes72
expect fail "a 90-character prose body line" "fix: a

${prose90}"
expect pass "a body line at exactly 72 characters" "fix: a

${prose72}"
expect pass "72 characters that are not 72 bytes" "fix: a

${dashes72}"
expect pass "a long line holding a URL" "fix: a

See https://example.com/${prose90// /-} for the rest."
expect pass "long trailers" "fix: a

Body.

Co-Authored-By: ${prose90// /-} <noreply@example.com>
Reviewed-by: ${prose90// /-}"
expect fail "a long line in a last paragraph that is not all trailers" "fix: a

Body.

Note: ${prose90}
and a second prose line."
expect pass "an indented block" "fix: a

The output was:

    ${prose90}"
expect pass "a fenced block" "fix: a

\`\`\`text
${prose90}
\`\`\`"
expect fail "prose after a fenced block closes" "fix: a

\`\`\`text
x
\`\`\`

${prose90}"

# Dependabot writes its own body, which no setting rewraps, so a body it signs
# is left as it is. This is f06e1fb's message, verbatim.
expect pass "a body Dependabot signed" "build: Bump ghcr.io/devcontainers/features/docker-in-docker

Bumps ghcr.io/devcontainers/features/docker-in-docker from 2.17.0 to 4.1.1.

---
updated-dependencies:
- dependency-name: ghcr.io/devcontainers/features/docker-in-docker
  dependency-version: 4.1.1
  dependency-type: direct:production
  update-type: version-update:semver-major
...

Signed-off-by: dependabot[bot] <support@github.com>
"
expect fail "the same body signed by a person" "build: Bump ghcr.io/devcontainers/features/docker-in-docker

Bumps ghcr.io/devcontainers/features/docker-in-docker from 2.17.0 to 4.1.1.

Signed-off-by: A Person <a@example.com>"
expect fail "a Dependabot sign-off that is not a trailer" "fix: a

${prose90}

Signed-off-by: dependabot[bot] <support@github.com>
and a second prose line."

# Under `git commit -v` the message file carries the diff below a scissors line,
# and git's boilerplate comment lines above it; neither is the commit message,
# so a BREAKING-CHANGE in a diff hunk must not refuse the commit.
expect pass "a diff below the scissors line is ignored" "$(printf 'feat: add a thing\n\nA real body.\n# ------------------------ >8 ------------------------\ndiff --git a/x b/x\n+BREAKING-CHANGE: in the diff, not the message')"
expect pass "a commented example is ignored" "$(printf 'feat: add a thing\n\nReal body.\n# BREAKING-CHANGE: only an example in a comment')"

# The message that made release-please propose 0.1.0 for a docs change: a
# wrapped sentence left "BREAKING CHANGE:" at the start of a line.
expect fail "a wrapped sentence that reads as a breaking footer" "docs: correct the pre-1.0 version bump rules

CONTRIBUTING.md claimed \"feat bumps the minor version\", which is the
opposite of what release-please is configured to do and what this
project wants. Before 1.0 the minor digit is reserved for breaking
changes: feat and fix both bump the patch, and only feat! or a
BREAKING CHANGE: footer bumps the minor.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
expect fail "the hyphenated footer without !" "feat: drop the old flag

BREAKING-CHANGE: --old is gone"
expect fail "the # footer form without !" "feat: drop the old flag

BREAKING CHANGE #12"

expect fail "! only in the description" "fix: stop crashing!

BREAKING CHANGE: the old flag is gone"

# release-please 17.6.0 marks each of these breaking too, through paths that
# do not start a line with the footer; each was checked against its parser.
expect fail "the hyphenated footer mid-line" "docs: explain version bumps

Only feat! or a BREAKING-CHANGE: footer bumps the minor."
expect fail "a footer indented under another trailer" "fix: stop the retry loop

Refs: #12, which explains why this is not a
  BREAKING CHANGE: nothing depended on it"
expect fail "a footer indented with a no-break space" "$(printf 'fix: a\n\nRefs: x\n\302\240BREAKING CHANGE: b')"
expect fail "a token!: footer" "docs: tidy wording

See the rules.
Note!: nothing breaks here"
expect fail "an example subject at the end of the body" "docs: show a breaking subject

For example:

feat(cli)!: rename doctor to check

Co-Authored-By: A <a@example.com>"
expect fail "a paragraph release-please splits off" "fix: a

feat(x)!: b (c): d

Plain body paragraph."
expect fail "a nested commit mid-sentence" "fix: a

See BEGIN_NESTED_COMMIT feat!: b END_NESTED_COMMIT here."
expect fail "a footer after a bare carriage return" "$(printf 'fix: a\r\rBREAKING CHANGE: b')"

expect pass "CRLF line endings" "$(printf 'feat: a\r\n\r\nBody.\r\n')"
expect pass "a trailer with parentheses" "fix: a

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
expect pass "a breaking change marked both ways" "feat!: drop the old flag

BREAKING CHANGE: --old is gone"
expect pass "a scoped breaking change marked both ways" "fix(config)!: reject unknown keys

BREAKING-CHANGE: a misspelled key is now an error"
expect pass "a breaking subject with no footer" "feat(cli)!: rename doctor to check"
expect pass "the phrase mid-line" "docs: explain version bumps

Only feat! or a BREAKING CHANGE: footer bumps the minor."

finish_tests
