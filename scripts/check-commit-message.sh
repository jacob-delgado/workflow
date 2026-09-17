#!/usr/bin/env bash
#
# Fail if a commit message does not follow Conventional Commits.
#
# Usage:
#   scripts/check-commit-message.sh <message-file>
#
# lefthook's commit-msg hook and CI's commit check both run this, so a
# LEFTHOOK=0 bypass cannot land a message the hook would have refused. The
# first line it prints on a refusal is the reason, for CI's annotation.
#
# A breaking change is marked in the subject with "!", and a body may only say
# so as well. release-please (17.6.0, checked against its parser) also marks a
# release breaking from the body alone, in ways a wrapped sentence or an example
# can trigger by accident, and before 1.0 that bumps the minor version. So
# unless the subject has the "!", the body may not contain any of them:
#
#   BREAKING CHANGE: or BREAKING-CHANGE: (or " #") starting a line, including
#     indented, which is how a footer continues another footer
#   BREAKING-CHANGE: anywhere, which release-please matches unanchored
#   a line shaped like a breaking subject or footer, token!: or type(scope)!:
#
# and no body may contain BEGIN_NESTED_COMMIT, which makes the rest of the
# message separate commits. release-please also breaks lines at a bare carriage
# return, so this does too.
set -euo pipefail

readonly subject_pattern='^(feat|fix|chore|docs|refactor|test|perf|build|ci|revert|style)(\([a-z0-9_-]+\))?!?: .+'
readonly breaking_subject_pattern='^[a-z]+(\([a-z0-9_-]+\))?!: '

# What release-please's parser counts as whitespace before a continuation:
# space, tab, vertical tab, form feed, no-break space and zero-width no-break
# space.
leading="^([[:blank:]]|"$'\v|\f|\302\240|\357\273\277'")*"
readonly leading
readonly breaking_footer_pattern="${leading}BREAKING[ -]CHANGE(:| #)"
readonly hyphenated_footer_pattern='BREAKING-CHANGE:'
readonly breaking_token_pattern="${leading}[^[:space:]():!]+(\\(.*\\))?!(:| #)"

usage() {
  sed -n '3,6p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
}

# refuse prints a reason, then any detail, and fails.
refuse() {
  printf '%s\n' "$@" >&2
  exit 1
}

if [[ $# -ne 1 || ! -f "$1" ]]; then
  usage
  exit 2
fi

message="$(tr '\r' '\n' <"$1")"
subject="${message%%$'\n'*}"
body=""
if [[ "${message}" == *$'\n'* ]]; then
  body="${message#*$'\n'}"
fi

if ! grep -qE "${subject_pattern}" <<<"${subject}"; then
  refuse "Not a Conventional Commit subject: ${subject}" \
    "  <type>(<scope>)?(!)?: <description>" \
    "  e.g. feat(jira): add issue transition command" \
    "       fix(config): reject unknown keys instead of ignoring them" \
    "Allowed types: feat, fix, chore, docs, refactor, test, perf, build, ci, revert, style"
fi

if grep -q 'BEGIN_NESTED_COMMIT' <<<"${body}"; then
  refuse "The body contains BEGIN_NESTED_COMMIT, which release-please reads as separate commits" \
    "  Split the change into real commits instead."
fi

if grep -qE "${breaking_subject_pattern}" <<<"${subject}"; then
  exit 0
fi

if grep -qE "${breaking_footer_pattern}|${hyphenated_footer_pattern}|${breaking_token_pattern}" <<<"${body}"; then
  refuse "The body marks the release breaking, but the subject has no \"!\": ${subject}" \
    "  release-please reads a breaking change from a body line that starts with" \
    "  BREAKING CHANGE: or looks like token!:, and from BREAKING-CHANGE: anywhere." \
    "  A real breaking change: mark the subject too, as in feat!: or fix(config)!:" \
    "  A sentence or example that landed there: reword or rewrap it"
fi
