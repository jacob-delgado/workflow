#!/usr/bin/env bash
#
# Tests for which depguard rules check-layout.sh reads as holding a package
# marked "(tests only)" to tests: only a rule under
# linters.settings.depguard.rules in .golangci.yml whose files leave out
# "$test" and nothing else, or besides it only a directory every package of
# which a rule holds to tests in turn, whose deny list names the package and
# whose list-mode, if set, is original, while a rule of any other shape or
# place holds nothing. How the gate reads the YAML the rules are written in is
# check-layout_yaml_test.sh's, and the layout block itself, and the mark's
# need of a rule, are check-layout_test.sh's.
#
# Usage:
#   scripts/check-layout_depguard_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-layout.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-layout-fixtures.sh
source "${here}/lib/check-layout-fixtures.sh"

# fixed_rules prints, at the indent of depguard's rules, the rules the cases
# below mark packages against, each under a comment saying what it denies, and
# to which files. The gate must read none of internal/webserver/webtest,
# internal/ptytest, internal/oldfake, internal/stalefake, internal/fakebus,
# internal/fakerss, internal/fakenntp, internal/fakegopher, internal/fakewiki,
# internal/fakeci, internal/fakeoauth, internal/fakemetrics, internal/fakeping,
# internal/fakepong and internal/fakedns as held to tests, nor
# internal/fakehttp but where a rule holds every package under a directory
# named internal/fakekit, and every other package they name as held.
fixed_rules() {
  cat <<'END'
        # Denied in internal/loop alone.
        webtest-out-of-the-loop:
          files:
            - "**/internal/loop/**"
            - "!$test"
          deny:
            - pkg: example/internal/webserver/webtest
              desc: not in the loop
        # Denied everywhere but internal/tui.
        ptytest-in-the-interface:
          files:
            - "$all"
            - "!$test"
            - "!**/internal/tui/**"
          deny:
            - pkg: example/internal/ptytest
              desc: a terminal only the interface opens
        # Denied to every file, the tests as well.
        oldfake-nowhere:
          files:
            - "$all"
          deny:
            - pkg: example/internal/oldfake
              desc: a retired fake
        # Denied to every file but a test: depguard reads a files list of
        # negations alone as every file, less those.
        fakeclock-only-in-tests:
          files:
            - "!$test"
          deny:
            - pkg: example/internal/fakeclock
              desc: a test helper; use it only from a _test.go file
        # Denied to every file, the tests as well: allow names packages.
        stalefake-nowhere:
          files:
            - "$all"
          allow:
            - $gostd
            - "!$test"
          deny:
            - pkg: example/internal/stalefake
              desc: a retired fake
        # Both denied to every file but a test, by one rule.
        fakesms-and-fakepush-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakesms
              desc: a test helper; use it only from a _test.go file
            - pkg: example/internal/fakepush
              desc: a test helper; use it only from a _test.go file
        # Denied to no file: depguard reads files of negations alone as every
        # file less those, and these leave out "$all" as well as the tests.
        fakenntp-nowhere:
          files:
            - "!$test"
            - "!$all"
          deny:
            - pkg: example/internal/fakenntp
              desc: a test helper; use it only from a _test.go file
        # Denied to every file but a test and a file under a directory named
        # $test: depguard expands "$test" only where it stands alone, so
        # "!$test/**" leaves out that directory, which is no test.
        fakegopher-only-in-tests:
          files:
            - "$all"
            - "!$test"
            - "!$test/**"
          deny:
            - pkg: example/internal/fakegopher
              desc: a test helper; use it only from a _test.go file
        # Denied to no file: depguard expands "$all" only where it stands
        # alone, so "$all/**" names the files under a directory named $all.
        fakewiki-nowhere:
          files:
            - "$all/**"
            - "!$test"
          deny:
            - pkg: example/internal/fakewiki
              desc: a test helper; use it only from a _test.go file
        # Denied to the tests alone: "$test" is every test, and no other file.
        fakeci-in-tests:
          files:
            - "$test"
          deny:
            - pkg: example/internal/fakeci
              desc: a test helper; use it only from a _test.go file
        # Denied to every file but a test, but in lax list mode, where an
        # allow item longer than the deny item it matches lets the import
        # through: the "$" that makes this one an exact match is one byte
        # more than the deny item's path.
        fakeoauth-lax:
          list-mode: lax
          files:
            - "$all"
            - "!$test"
          allow:
            - example/internal/fakeoauth$
          deny:
            - pkg: example/internal/fakeoauth
              desc: a test helper; use it only from a _test.go file
        # The same in strict list mode, where the longer allow item wins too.
        fakemetrics-strict:
          list-mode: strict
          files:
            - "$all"
            - "!$test"
          allow:
            - $gostd
            - example/internal/fakemetrics$
          deny:
            - pkg: example/internal/fakemetrics
              desc: a test helper; use it only from a _test.go file
        # Denied to every file but a test in depguard's original list mode,
        # the one it reads when none is set, and whose name it lowers, where
        # a deny item wins over any allow item.
        fakevault-only-in-tests:
          list-mode: Original
          files:
            - "$all"
            - "!$test"
          allow:
            - example/internal/fakevault$
          deny:
            - pkg: example/internal/fakevault
              desc: a test helper; use it only from a _test.go file
        # Denied to every file but a test, a deny item's desc first: its pkg
        # is internal/fakeatom, and its desc names internal/fakerss.
        fakeatom-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - desc: example/internal/fakerss
              pkg: example/internal/fakeatom
        # Denied to every file but a test and a file under a directory named
        # internal/fakekit, which a case's own rule may hold to tests in turn.
        fakehttp-in-tests-and-the-kit:
          files:
            - "$all"
            - "!$test"
            - "!**/internal/fakekit/**"
          deny:
            - pkg: example/internal/fakehttp
              desc: a test helper; use it only from a _test.go file or the kit
        # Denied to every file but a test and internal/fakepong's, whose rule
        # leaves out internal/fakeping's in turn: neither is held first.
        fakeping-in-tests-and-pong:
          files:
            - "$all"
            - "!$test"
            - "!**/internal/fakepong/**"
          deny:
            - pkg: example/internal/fakeping
              desc: a test helper; use it only from a _test.go file or pong
        fakepong-in-tests-and-ping:
          files:
            - "$all"
            - "!$test"
            - "!**/internal/fakeping/**"
          deny:
            - pkg: example/internal/fakepong
              desc: a test helper; use it only from a _test.go file or ping
        # Denied to every file but a test and one under any directory whose
        # name begins internal/fakekit: a pattern, not one directory.
        fakedns-in-tests-and-kits:
          files:
            - "$all"
            - "!$test"
            - "!**/internal/fakekit*/**"
          deny:
            - pkg: example/internal/fakedns
              desc: a test helper; use it only from a _test.go file or a kit
        # Denied to every file but a test. Its allow list holds a string that
        # reads like a deny item for internal/fakebus, but allow is a list of
        # strings, and the rule denies fakebus nothing by name.
        fakequeue-only-in-tests:
          files:
            - "$all"
            - "!$test"
          allow:
            - "pkg: example/internal/fakebus"
          deny:
            - pkg: example/internal/fakequeue
              desc: a test helper; use it only from a _test.go file
END
}

# misplaced_rules prints the rest of a .golangci.yml after depguard's rules:
# rules of the held shape where depguard reads no rule, for
# internal/fakejira, internal/fakeforge, internal/fakeslack and
# internal/faketeams, under depguard's rule, under gomodguard, under linters'
# setting and under formatters' settings. The gate must read none of them as
# held to tests.
misplaced_rules() {
  cat <<'END'
      # depguard's shape, under rule: a typo for rules, which depguard does
      # not read.
      rule:
        fakejira-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakejira
              desc: a test helper; use it only from a _test.go file
    # depguard's shape, under a linter that is not depguard.
    gomodguard:
      rules:
        fakeforge-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakeforge
              desc: a test helper; use it only from a _test.go file
  # depguard's shape, under setting: a typo for settings, which golangci-lint
  # does not read.
  setting:
    depguard:
      rules:
        fakeslack-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakeslack
              desc: a test helper; use it only from a _test.go file

# depguard's shape, under formatters, which hold no depguard.
formatters:
  settings:
    depguard:
      rules:
        faketeams-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/faketeams
              desc: a test helper; use it only from a _test.go file
END
}

# lint writes, in a repository repo made, the go.mod of a module named example
# and a .golangci.yml that turns depguard on, as the root's does, whose
# depguard rules are the fixed rules, then one holding each package named to
# tests, as the root's *-only-in-tests rules do.
#   lint <dir> <package-dir>...
lint() {
  local dir="$1"
  shift
  module "${dir}"
  {
    depguard_rules_head "default: all"
    fixed_rules
    rules_holding "$@"
  } >"${dir}/.golangci.yml"
}

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  expect_exit "$1" "$2" run_in "$3" "${check}"
}

# depguard reads a rule whose files are "!$test" alone as every file but a
# test, so such a rule holds its package as the usual "$all" and "!$test" do.
negated="${workdir}/negated"
repo "${negated}" "internal/fakeclock (tests only)" -- internal/fakeclock/clock.go
lint "${negated}"
expect pass "a marked package whose rule has the files \"!\$test\" alone" "${negated}"

# A rule that denies two packages holds each, the first as well as the last.
twice="${workdir}/twice"
repo "${twice}" "internal/fakesms (tests only)" "internal/fakepush (tests only)" \
  -- internal/fakesms/sms.go internal/fakepush/push.go
lint "${twice}"
expect pass "marked packages one rule's deny list names both of" "${twice}"

# depguard reads a list mode of original, in any case, as it reads none: a
# deny item wins over an allow item however long, so the rule holds its
# package.
original="${workdir}/original"
repo "${original}" "internal/fakevault (tests only)" -- internal/fakevault/vault.go
lint "${original}"
expect pass "a marked package whose rule's list-mode is Original" "${original}"

# A rule may leave out a directory besides the tests, "!**/<dir>/**", when
# every package whose files that leaves out is one a rule holds to tests in
# turn: a test helper may import another, since only tests reach either.
# The gate sorts the denials, and example/internal/fakehttp sorts before
# example/internal/fakekit, so its first pass meets fakehttp before the kit
# is held and only a second pass holds it: a rename that sorts the kit first
# stops this case from testing that a chain holds.
kit="${workdir}/kit"
repo "${kit}" "internal/fakehttp (tests only)" "internal/fakekit (tests only)" \
  -- internal/fakehttp/http.go internal/fakekit/kit.go
lint "${kit}" internal/fakekit
expect pass "a marked package whose rule leaves out a directory a rule holds to tests" "${kit}"

# "**/internal/fakekit/**" leaves out every directory of that name, at any
# depth, so a package under another of them that no rule holds is free to
# import internal/fakehttp, and the rule holds it to nothing.
vendored="${workdir}/vendored"
repo "${vendored}" "internal/fakehttp (tests only)" "internal/fakekit (tests only)" \
  internal/vendored/internal/fakekit \
  -- internal/fakehttp/http.go internal/fakekit/kit.go internal/vendored/internal/fakekit/kit.go
lint "${vendored}" internal/fakekit
expect_output fail "a marked package whose rule leaves out a directory a package no rule holds is under too" \
  "internal/fakehttp/ (pkg: example/internal/fakehttp)" run_in "${vendored}" "${check}"

# The gate reads a directory left out only as "!**/<dir>/**" with no pattern
# in <dir>, so a rule leaving out every directory "internal/fakekit*" names
# holds nothing, though a rule holds internal/fakekit itself to tests.
kits="${workdir}/kits"
repo "${kits}" "internal/fakedns (tests only)" "internal/fakekit (tests only)" \
  -- internal/fakedns/dns.go internal/fakekit/kit.go
lint "${kits}" internal/fakekit
expect_output fail "a marked package whose rule leaves out a pattern of directories" \
  "internal/fakedns/ (pkg: example/internal/fakedns)" run_in "${kits}" "${check}"

# Two rules that each leave out the other's directory hold neither package:
# each is held only once the other is.
cycle="${workdir}/cycle"
repo "${cycle}" "internal/fakeping (tests only)" "internal/fakepong (tests only)" \
  -- internal/fakeping/ping.go internal/fakepong/pong.go
lint "${cycle}"
expect_output fail "marked packages whose rules each leave out the other's directory" \
  $'  internal/fakeping/ (pkg: example/internal/fakeping)\n  internal/fakepong/ (pkg: example/internal/fakepong)' \
  run_in "${cycle}" "${check}"

# A rule that leaves some file other than a test free to import the package
# does not hold it to tests, whether it denies it in one directory alone or
# everywhere but one, or leaves out a file besides the tests with a "!$all" or
# a "!$test/**" that only begins as "!$test" does, or names no file but under
# a directory named $all with an "$all/**" that only begins as "$all" does, or
# leaves out a directory no rule holds to tests; nor does one whose files are
# "$test" alone, the tests and no other file; nor does one in lax or strict
# list mode, where an allow item longer than the deny item lets the import
# through; nor does one that denies it to the tests as well, though a "!$test"
# stands in its allow list rather than its files; nor does a rule of the right
# shape anywhere but linters.settings.depguard.rules, where golangci-lint
# reads depguard's rules: under a linter that is not depguard, under
# depguard's rule, linters' setting or formatters' settings; nor does a string
# in an allow list that reads like a deny item, or a deny item's desc. Each
# package fails by name, though a rule that holds internal/rlimit follows
# them.
scoped="${workdir}/scoped"
repo "${scoped}" "internal/webserver/webtest (tests only)" "internal/ptytest (tests only)" \
  "internal/oldfake (tests only)" "internal/stalefake (tests only)" \
  "internal/fakeforge (tests only)" "internal/fakebus (tests only)" \
  "internal/fakejira (tests only)" "internal/fakeslack (tests only)" \
  "internal/faketeams (tests only)" "internal/fakenntp (tests only)" \
  "internal/fakegopher (tests only)" "internal/fakerss (tests only)" \
  "internal/fakewiki (tests only)" "internal/fakeci (tests only)" \
  "internal/fakeoauth (tests only)" "internal/fakemetrics (tests only)" \
  "internal/fakehttp (tests only)" "internal/fakekit" \
  -- internal/webserver/webtest/serve.go internal/ptytest/open.go internal/oldfake/fake.go \
  internal/stalefake/fake.go internal/fakeforge/forge.go internal/fakebus/bus.go \
  internal/fakejira/jira.go internal/fakeslack/slack.go internal/faketeams/teams.go \
  internal/fakenntp/nntp.go internal/fakegopher/gopher.go internal/fakerss/rss.go \
  internal/fakewiki/wiki.go internal/fakeci/ci.go internal/fakeoauth/oauth.go \
  internal/fakemetrics/metrics.go internal/fakehttp/http.go internal/fakekit/kit.go
lint "${scoped}" internal/rlimit
misplaced_rules >>"${scoped}/.golangci.yml"
expect_output fail "a marked package a rule denies in one directory alone" \
  "internal/webserver/webtest/ (pkg: example/internal/webserver/webtest)" run_in "${scoped}" "${check}"
expect_output fail "a marked package a rule denies everywhere but one directory" \
  "internal/ptytest/ (pkg: example/internal/ptytest)" run_in "${scoped}" "${check}"
expect_output fail "a marked package a rule denies everywhere but a directory no rule holds" \
  "internal/fakehttp/ (pkg: example/internal/fakehttp)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule's files say \"!\$all\" too" \
  "internal/fakenntp/ (pkg: example/internal/fakenntp)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule's files say \"!\$test/**\" too" \
  "internal/fakegopher/ (pkg: example/internal/fakegopher)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule's files say \"\$all/**\" for \"\$all\"" \
  "internal/fakewiki/ (pkg: example/internal/fakewiki)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule's files are \"\$test\" alone" \
  "internal/fakeci/ (pkg: example/internal/fakeci)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule is in lax list mode" \
  "internal/fakeoauth/ (pkg: example/internal/fakeoauth)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule is in strict list mode" \
  "internal/fakemetrics/ (pkg: example/internal/fakemetrics)" run_in "${scoped}" "${check}"
expect_output fail "a marked package a rule denies to the tests as well" \
  "internal/oldfake/ (pkg: example/internal/oldfake)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule has \"!\$test\" in its allow list, not its files" \
  "internal/stalefake/ (pkg: example/internal/stalefake)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule sits under another linter" \
  "internal/fakeforge/ (pkg: example/internal/fakeforge)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule sits under depguard's rule, not rules" \
  "internal/fakejira/ (pkg: example/internal/fakejira)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule sits under linters' setting, not settings" \
  "internal/fakeslack/ (pkg: example/internal/fakeslack)" run_in "${scoped}" "${check}"
expect_output fail "a marked package whose rule sits under formatters, not linters" \
  "internal/faketeams/ (pkg: example/internal/faketeams)" run_in "${scoped}" "${check}"
expect_output fail "a marked package only a string in a rule's allow list names" \
  "internal/fakebus/ (pkg: example/internal/fakebus)" run_in "${scoped}" "${check}"
expect_output fail "a marked package only a deny item's desc names" \
  "internal/fakerss/ (pkg: example/internal/fakerss)" run_in "${scoped}" "${check}"

finish_tests
