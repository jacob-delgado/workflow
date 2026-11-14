#!/usr/bin/env bash
#
# Tests for gobco-report.sh's check of the lines a change touched, run against
# the stand-ins lib/gobco-report-fixtures.sh makes: a condition seen one way
# on a line the change added or modified fails the run, by file, line and
# function, and says why, judged against the merge base with GATE_BASE, its
# file named by a path from the repository's root as gobco names it or by an
# absolute one; one on a line the change left as it was, the line just after
# a changed one among them, or one the base alone changed after the fork, is
# only listed; a line changed in the working tree alone counts as changed; an
# arm the allowlist keeps passes on a changed line too; and an arm no test
# reached on one fails as it did before.
#
# Where GATE_BASE is set empty, as CI sets it on a push to main, the run says
# it judges no line and passes; where it is unset and the clone holds no
# origin/main, or no merge base with it, the run says so, skips that check
# alone, and passes; and where GATE_BASE names a base the clone does not
# hold, or shares no commit with, the run fails, saying so. CI's Branch
# coverage job names a pull request's base, and none on a push, from a
# checkout of the whole history.
#
# Usage:
#   scripts/gobco-report_changes_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly report="${here}/gobco-report.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/gobco-report-fixtures.sh
source "${here}/lib/gobco-report-fixtures.sh"

# The fixture is the head of a change: its base branch holds sender.send as it
# was, and the change, forked from it, modifies the function's first condition
# (line 70) and adds an error check below it (line 74), leaving the third
# (line 78) as it was. The base then moves on past the fork to change ready's
# line (13), which the change does not touch, so only a report that judged
# the base's own head would read it as changed.
#
# sender writes sender.send below the rest of the fixture, its first
# condition the one given, and with the error check when asked.
#   sender <first condition> [flush]
sender() {
  {
    printf '\nfunc (s *sender) send(line string) bool {\n\tif %s {\n\t\treturn false\n\t}\n\n' "$1"
    if [[ -n "${2:-}" ]]; then
      printf '\tif err := s.flush(); err != nil {\n\t\treturn false\n\t}\n\n'
    fi
    printf '\tif s.ready && line != "" {\n\t\treturn s.write(line)\n\t}\n\n\treturn s.full()\n}\n'
  } >>"${fixture}"
}
cp "${fixture}" "${workdir}/reader-head.go"
sender s.done
commit base
git -C "${fixture_repo}" switch --quiet --create change
cp "${workdir}/reader-head.go" "${fixture}"
sender s.closed flush
commit change
git -C "${fixture_repo}" switch --quiet base
sed -i '13s/open && err == nil/err == nil \&\& open/' "${fixture}"
commit "base moves on"
git -C "${fixture_repo}" switch --quiet change

modified_one_way="[$(condition 70:5 's.closed' 0 3)]"
added_one_way="[$(condition 74:23 'err != nil' 2 0)]"

# A condition seen one way on a line the change modified or added fails,
# judged against the merge base with GATE_BASE, and the run says why.
expect fail "a condition seen one way on a line the change modified" "${modified_one_way}" "" \
  "${fixture}:70:5 in sender.send: \"s.closed\", number 1 there, was seen only false, on a line this change touched" \
  --package "${buildinfo}" 50
expect fail "a condition seen one way on a line the change added" "${added_one_way}" "" \
  "${fixture}:74:23 in sender.send: \"err != nil\", number 1 there, was seen only true, on a line this change touched" \
  --package "${buildinfo}" 50
expect fail "a condition seen one way on a changed line, and why that fails" "${modified_one_way}" "" \
  "Conditions seen only one way on a line this change touched, since base (merge base" \
  --package "${buildinfo}" 50

# gobco names a file by its path from the repository's root, which is read
# and matched under that root.
expect fail "a condition seen one way on a changed line, named from the root" \
  "[$(condition_in reader.go 70:5 's.closed' 0 3)]" "" \
  "reader.go:70:5 in sender.send: \"s.closed\", number 1 there, was seen only false, on a line this change touched" \
  --package "${buildinfo}" 50

# One on a line the change left as it was, or that only the base changed
# after the fork, is the worklist's.
expect pass "a condition seen one way on a line the change left as it was" "[$(condition 78:5 's.ready' 3 0)]" "" \
  "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50
expect pass "a condition seen one way on a line only the base changed after the fork" \
  "[$(condition 13:9 'open' 4 0)]" "" "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50

# gobco measures the working tree, so a line changed there and not yet
# committed counts as changed, and the line just after a changed one does not:
# a clone of the change whose working tree turns the blank line above the
# third condition (77) into a comment and adds a condition to the return
# (82).
edited="${workdir}/edited"
git clone --quiet --branch=change "file://${fixture_repo}" "${edited}"
sed -i -e '77s/^$/\t\/\/ A line to write./' -e '82s/s.full()/s.full() || s.closed/' "${edited}/reader.go"
GOBCO_CHANGES_ROOT="${edited}" GATE_BASE=origin/base expect fail "a condition seen one way on a line changed in the working tree" \
  "[$(condition_in reader.go 82:21 's.closed' 0 2)]" "" \
  "reader.go:82:21 in sender.send: \"s.closed\", number 1 there, was seen only false, on a line this change touched" \
  --package "${buildinfo}" 50
GOBCO_CHANGES_ROOT="${edited}" GATE_BASE=origin/base expect pass "a condition seen one way on the line after a changed one" \
  "[$(condition_in reader.go 78:5 's.ready' 3 0)]" "" "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50

# An arm the allowlist keeps passes on a changed line too, and an arm no test
# reached fails there as anywhere.
flushed="$(allowlist flushed "${fixture} sender.send TRADE-1 any 1 err != nil")"
GOBCO_ALLOWLIST="${flushed}" expect pass "an error arm the allowlist keeps, on a changed line" \
  "[$(condition 74:23 'err != nil' 0 2)]" "" "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50
sent="$(allowlist sent "${fixture} sender.send TRADE-1 any 1 err != nil" "${fixture} sender.send TRADE-2 any 1 s.closed")"
GOBCO_ALLOWLIST="${sent}" expect pass "a condition seen one way the allowlist keeps, on a changed line" \
  "[$(condition 70:5 's.closed' 0 3),$(condition 74:23 'err != nil' 0 2)]" "" \
  "Condition coverage 50.0% (floor 50%)." --package "${buildinfo}" 50
expect fail "an error arm no test reached, on a changed line" "[$(condition 74:23 'err != nil' 0 2)]" "" \
  "${fixture}:74:23 in sender.send: \"err != nil\", number 1 there, was never seen true, the error's arm" \
  --package "${buildinfo}" 50
expect fail "a condition never evaluated, on a changed line" "[$(condition 70:5 's.closed' 0 0)]" "" \
  "${fixture}:70:5 in sender.send: \"s.closed\", number 1 there, was never evaluated" --package "${buildinfo}" 50

# GATE_BASE set empty, as on a push to main, judges no line, and says so.
GATE_BASE="" expect pass "GATE_BASE set empty" "${modified_one_way}" "" \
  "GATE_BASE is empty, so this change's lines are judged against no base: no condition seen one way fails for its line" \
  --package "${buildinfo}" 50

# Unset, the base is origin/main, and a clone that holds none, or a shallow
# one that holds it with no commit shared, says so and skips that check alone.
GATE_BASE_UNSET=1 expect pass "no origin/main, GATE_BASE unset" "${modified_one_way}" "" \
  "This clone holds no origin/main, so no condition seen one way fails for its line" --package "${buildinfo}" 50
shallow="${workdir}/shallow"
git clone --quiet --depth=1 --branch=change "file://${fixture_repo}" "${shallow}"
git -C "${shallow}" fetch --quiet --depth=1 origin base:refs/remotes/origin/main
GATE_BASE_UNSET=1 GOBCO_CHANGES_ROOT="${shallow}" expect pass "an origin/main with no commit shared, GATE_BASE unset" \
  "${modified_one_way}" "" \
  "origin/main shares no commit with HEAD here, as in a shallow clone, so no condition seen one way fails for its line" \
  --package "${buildinfo}" 50

# A base GATE_BASE names must be there to judge the change against, so a pull
# request whose checkout lost it fails rather than pass unjudged.
GATE_BASE=nowhere expect fail "a named base the clone does not hold" "[$(condition 78:5 's.ready' 3 0)]" "" \
  "GATE_BASE names nowhere. This clone holds no nowhere, so the lines this change touched cannot be judged." \
  --package "${buildinfo}" 50
GATE_BASE=origin/main GOBCO_CHANGES_ROOT="${shallow}" expect fail "a named base with no commit shared" \
  "[$(condition 78:5 's.ready' 3 0)]" "" \
  "GATE_BASE names origin/main. origin/main shares no commit with HEAD here, as in a shallow clone" \
  --package "${buildinfo}" 50

# CI's Branch coverage job checks out the whole history, and names a pull
# request's base branch as origin/<branch>, and none on a push to main.
job="$(awk '/^  branch-coverage:$/ { on = 1; next } on && /^  [a-z-]+:$/ { exit } on' "${here}/../.github/workflows/ci.yml")"
count_case
if [[ "${job}" != *"fetch-depth: 0"* ]]; then
  fail_case "CI's Branch coverage job" "its checkout does not fetch the whole history"
fi
count_case
# shellcheck disable=SC2016 # GitHub's expression, matched as written
if [[ "${job}" != *'GATE_BASE: ${{ github.event_name == '"'pull_request'"' && format('"'origin/{0}'"', github.base_ref) || '"''"' }}'* ]]; then
  fail_case "CI's Branch coverage job" "it does not name a pull request's base branch, and none on a push"
fi

finish_tests
