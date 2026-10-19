#!/usr/bin/env bash
#
# Tests for check-dependency-age.sh: a version a change adds to go.mod,
# web/yarn.lock or mise.toml passes once it was published at least seven days
# before, and fails, by name, when it is younger, unless the exceptions file
# lists it with the advisory it fixes or a maintainer's dated waiver; now is
# 2026-10-08T00:00:00Z, and a version exactly seven days old is adopted. An
# exception naming neither, or a waiver whose date is no real day, fails. A
# version whose publish date cannot be learned fails, as a mise tool pinned in
# a [tools.<name>] table, which the check does not read, or in an inline table
# naming no version, does; one pinned in an inline table is dated by its
# version key. Versions the change leaves alone are not looked up, and a base
# git cannot resolve is refused with a status of its own. A dated waiver a week
# past its day is warned of, and its npmPreapprovedPackages entry in
# web/.yarnrc.yml with it, but fails nothing; one whose week may not be over is
# not. In GitHub Actions the warning is also a ::warning annotation, which the
# pull request shows. A pre-approval no line of the exceptions file lists is
# warned of too, and fails nothing. The linters' pin files are
# check-dependency-age_pins_test.sh's.
#
# The stand-in curl lib/check-dependency-age-fixtures.sh writes answers each
# registry URL from a fixture file named for it, and fails as curl -f does for
# any other, so no case reaches the network.
#
# Usage:
#   scripts/check-dependency-age_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-dependency-age-fixtures.sh
source "${here}/lib/check-dependency-age-fixtures.sh"

# require_go adds a requirement to go.mod.
require_go() {
  sed -i.bak "s|^)|\t$1 $2\n)|" go.mod && rm go.mod.bak
}

# resolve_npm adds a resolution to web/yarn.lock.
resolve_npm() {
  printf '\n"%s@npm:^%s":\n  version: %s\n  resolution: "%s@npm:%s"\n' "$1" "$2" "$2" "$1" "$2" >>web/yarn.lock
}

# pin_tool adds or changes a [tools] pin in mise.toml.
pin_tool() {
  printf '"%s" = "%s"\n' "$1" "$2" >>mise.toml
}

# young_go_with_advisory requires the young module and lists it as a fix.
young_go_with_advisory() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0 GHSA-abcd-1234-wxyz
}

# young_go_without_advisory lists the young module with no advisory.
young_go_without_advisory() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0
}

# young_go_waived requires the young module and lists it as waived.
young_go_waived() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0 waived-2026-10-07
}

# young_go_waived_on_no_day lists the young module as waived on a date no
# calendar holds.
young_go_waived_on_no_day() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0 waived-2026-02-30
}

# preapprove_npm lists an npm version in web/.yarnrc.yml's
# npmPreapprovedPackages, as that file spells an entry.
#   preapprove_npm <package> <version>
preapprove_npm() {
  printf "npmPreapprovedPackages:\n  # Waived by the maintainer.\n  - '%s@%s'\n\npackageExtensions: {}\n" "$1" "$2" \
    >web/.yarnrc.yml
}

# npm_waiver_past_its_week lists an npm version waived a week before now, in
# the exceptions file and in web/.yarnrc.yml.
npm_waiver_past_its_week() {
  except @scope/widget 2.0.0 waived-2026-09-30
  preapprove_npm @scope/widget 2.0.0
}

# npm_waiver_in_its_week lists an npm version waived the day before now, in
# the exceptions file and in web/.yarnrc.yml.
npm_waiver_in_its_week() {
  except @scope/widget 2.0.0 waived-2026-10-07
  preapprove_npm @scope/widget 2.0.0
}

# table_tool pins a tool in a [tools.<name>] table of its own, which mise
# reads as a [tools] pin.
table_tool() {
  printf '\n[tools.shellcheck]\nversion = "0.12.0"\n' >>mise.toml
}

# inline_tool pins a tool for some platforms only, in an inline table.
inline_tool() {
  printf 'shellcheck = { version = "0.12.0", os = ["linux"] }\n' >>mise.toml
}

# versionless_tool pins a tool in an inline table that names no version.
versionless_tool() {
  printf 'shellcheck = { os = ["linux"] }\n' >>mise.toml
}

# mise_tools pins mise tools of several kinds, all a week old.
mise_tools() {
  pin_tool node 24.22.0
  pin_tool go:github.com/rillig/gobco 1.4.0
  pin_tool shellcheck 0.12.0
}

go_old="$(repo go-old require_go example.com/old v1.2.0)"
expect_output pass "a Go module a week old" "example.com/old v1.2.0" age "${go_old}"

go_young="$(repo go-young require_go example.com/young v1.3.0)"
expect_output fail "a Go module two days old" "example.com/young v1.3.0 was published 1 day(s) ago" age "${go_young}"

answer "https://proxy.golang.org/example.com/week/@v/v1.0.0.info" '{"Time":"2026-10-01T00:00:00Z"}'
answer "https://proxy.golang.org/example.com/week/@v/v1.0.1.info" '{"Time":"2026-10-01T00:00:01Z"}'

go_week="$(repo go-week require_go example.com/week v1.0.0)"
expect_output pass "a Go module exactly seven days old" "example.com/week v1.0.0: published 2026-10-01" age "${go_week}"

go_week_less_a_second="$(repo go-week-less-a-second require_go example.com/week v1.0.1)"
expect_output fail "a Go module a second short of seven days old" \
  "example.com/week v1.0.1 was published 6 day(s) ago" age "${go_week_less_a_second}"

go_upper="$(repo go-upper require_go github.com/BurntSushi/toml v1.6.0)"
expect_exit pass "a Go module path with capitals, escaped for the proxy" age "${go_upper}"

go_fix="$(repo go-fix young_go_with_advisory)"
expect_output pass "a young Go module listed with its advisory" "GHSA-abcd-1234-wxyz" age "${go_fix}"

go_no_advisory="$(repo go-no-advisory young_go_without_advisory)"
expect_output fail "an exception that names no advisory" "names no advisory" age "${go_no_advisory}"

go_waived="$(repo go-waived young_go_waived)"
expect_output pass "a young Go module the maintainer waived" "waived on 2026-10-07" age "${go_waived}"

go_waived_on_no_day="$(repo go-waived-on-no-day young_go_waived_on_no_day)"
expect_output fail "a waiver dated on no real day" "names no advisory" age "${go_waived_on_no_day}"

npm_old="$(repo npm-old resolve_npm @scope/widget 2.0.0)"
expect_exit pass "a scoped npm package a week old" age "${npm_old}"

npm_young="$(repo npm-young resolve_npm left-pad 1.5.0)"
expect_output fail "an npm package two days old" "left-pad 1.5.0" age "${npm_young}"

go_toolchain="$(repo go-toolchain sed -i.bak 's/^go = "1.27.1"/go = "1.27.2"/' mise.toml)"
expect_output fail "a Go toolchain two days old" "go 1.27.2" age "${go_toolchain}"

tools_old="$(repo tools-old mise_tools)"
expect_exit pass "mise tools of each kind a week old" age "${tools_old}"

# A go: pin names the package `go install` builds, and the proxy knows its
# module, a few path elements up.
go_package="$(repo go-package pin_tool go:golang.org/x/vuln/cmd/govulncheck 1.9.0)"
expect_output fail "a Go-built tool whose module is up its package path" \
  "go:golang.org/x/vuln/cmd/govulncheck 1.9.0 was published 1 day(s) ago" age "${go_package}"

release_young="$(repo release-young pin_tool jq 1.9.0)"
expect_output fail "a GitHub release two days old, tagged with its name" "jq 1.9.0" age "${release_young}"

unknown="$(repo unknown pin_tool shellcheck 9.9.9)"
expect_output fail "a version whose publish date cannot be learned" "cannot tell when shellcheck 9.9.9 was published" \
  age "${unknown}"

unlisted="$(repo unlisted pin_tool newtool 1.0.0)"
expect_output fail "a mise tool with no known source" "newtool" age "${unlisted}"

tool_table="$(repo tool-table table_tool)"
expect_output fail "a tool pinned in a [tools.<name>] table, which is not read" \
  "mise.toml: '[tools.shellcheck]' cannot be dated" age "${tool_table}"

tool_inline="$(repo tool-inline inline_tool)"
expect_output pass "a tool pinned in an inline table, by its version" \
  "shellcheck 0.12.0: published 2026-09-20" age "${tool_inline}"

tool_versionless="$(repo tool-versionless versionless_tool)"
expect_output fail "a tool pinned in an inline table that names no version" \
  "mise.toml: 'shellcheck = { os = [\"linux\"] }' cannot be dated" age "${tool_versionless}"

untouched="$(repo untouched touch README.md)"
expect_exit pass "a change that adds no version" age "${untouched}"
count_case
if [[ -s "${untouched}.curl" ]]; then
  fail_case "a change that adds no version" "it looked up: $(cat "${untouched}.curl")"
fi

expect_exit 2 "a base git cannot resolve" age "${untouched}" no-such-branch

# Now is 2026-10-08: a version waived on 2026-09-30 was published by the end of
# that day, so it is a week old by now; one waived on 2026-10-01 may not be.
waiver_past_its_week="$(repo waiver-past-its-week except example.com/kept v1.0.0 waived-2026-09-30)"
expect_output pass "a waiver past its week, warned of" \
  "example.com/kept v1.0.0: waived on 2026-09-30, so a week old by now; remove its line from scripts/dependency-age-exceptions.txt" \
  age "${waiver_past_its_week}"
count_case
if grep -q "::warning" "${case_output}"; then
  fail_case "a waiver past its week, warned of" "it was annotated outside GitHub Actions: $(cat "${case_output}")"
fi

expect_output pass "a waiver past its week, annotated in GitHub Actions" \
  "::warning file=scripts/dependency-age-exceptions.txt,line=2::example.com/kept v1.0.0: waived on 2026-09-30, so a week old by now" \
  age_in_actions "${waiver_past_its_week}"
count_case
if ! grep -q "^warn  example.com/kept v1.0.0: waived on 2026-09-30" "${case_output}"; then
  fail_case "a waiver past its week, annotated in GitHub Actions" "its plain warning is gone: $(cat "${case_output}")"
fi

waiver_in_its_week="$(repo waiver-in-its-week except example.com/kept v1.0.0 waived-2026-10-01)"
expect_exit pass "a waiver whose week may not be over" age "${waiver_in_its_week}"
count_case
if grep -q "waived on" "${case_output}"; then
  fail_case "a waiver whose week may not be over" "it was warned of: $(cat "${case_output}")"
fi

npm_waiver="$(repo npm-waiver-past-its-week npm_waiver_past_its_week)"
expect_output pass "a pre-approved npm version past its week, warned of in both files" \
  "@scope/widget 2.0.0: waived on 2026-09-30, so a week old by now; remove its line from scripts/dependency-age-exceptions.txt and its npmPreapprovedPackages entry from web/.yarnrc.yml" \
  age "${npm_waiver}"

npm_waiver_listed="$(repo npm-waiver-in-its-week npm_waiver_in_its_week)"
expect_exit pass "a pre-approved npm version in its week, listed in the exceptions file" age "${npm_waiver_listed}"
count_case
if grep -q "warn" "${case_output}"; then
  fail_case "a pre-approved npm version in its week, listed in the exceptions file" \
    "it was warned of: $(cat "${case_output}")"
fi

preapproved_unlisted="$(repo preapproved-unlisted preapprove_npm @scope/widget 2.0.0)"
expect_output pass "a pre-approved npm version no line of the exceptions file lists, warned of" \
  "@scope/widget 2.0.0: web/.yarnrc.yml pre-approves it, but no line of scripts/dependency-age-exceptions.txt says why; remove its npmPreapprovedPackages entry" \
  age "${preapproved_unlisted}"

finish_tests
