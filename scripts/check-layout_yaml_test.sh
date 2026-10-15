#!/usr/bin/env bash
#
# Tests for how check-layout.sh reads .golangci.yml: as golangci-lint does,
# where it can be sure of that, and not at all where it cannot. It reads the
# first YAML document's block style a line at a time, through comments, blank
# lines, spaces, quotes, names that hold a "#", a ":" or a " - ", and lists at
# their key's indent; it reads each key in any case and split at each ".",
# as golangci-lint's configuration library does, so two keys it reads as one
# setting leave that setting unread; and it refuses, naming the line, any
# line it cannot read so. Which rules hold a package are
# check-layout_depguard_test.sh's, the mark's need of a depguard that runs,
# however its keys are written, check-layout_runs_test.sh's, and the layout
# block and the mark's need of a rule check-layout_test.sh's.
#
# Usage:
#   scripts/check-layout_yaml_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-layout.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-layout-fixtures.sh
source "${here}/lib/check-layout-fixtures.sh"

# alike_rules prints, at the indent of depguard's rules, rules each written in
# a way of its own that golangci-lint reads as it reads the root's, each under
# a comment saying how. Each holds its package to tests but
# internal/fakexmpp's, internal/fakemqtt's, internal/fakepop's,
# internal/fakedhcp's and internal/memstore's, which the comments say why.
alike_rules() {
  cat <<'END'
        # A comment beside a file, one of them holding a ":".
        fakemail-only-in-tests:
          files:
            - $all # every file: these
            - "!$test" # tests may
          deny:
            - pkg: example/internal/fakemail
        # A line commented out at the margin among the files.
        fakechat-only-in-tests:
          files:
            - "$all"
#           - "!**/internal/tui/**"
            - "!$test"
          deny:
            - pkg: example/internal/fakechat
        # A line commented out among the files at the indent of their key.
        fakefax-only-in-tests:
          files:
            - "$all"
          # - "!**/internal/tui/**"
            - "!$test"
          deny:
            - pkg: example/internal/fakefax

        # A blank line above the rule's comment, a comment beside its files
        # key.
        fakecache-only-in-tests:
          files: # every file but a test
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakecache
        # Two rules, a "#" in each name: fake#2 denies internal/fakexmpp to
        # the tests as well.
        fake#1:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakeirc
        fake#2:
          files:
            - "$all"
          deny:
            - pkg: example/internal/fakexmpp
        # Two rules, a ":" in each name: fake:2 denies internal/fakemqtt to
        # the tests as well.
        fake:1:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakesip
        fake:2:
          files:
            - "$all"
          deny:
            - pkg: example/internal/fakemqtt
        # Two rules, quoted names that differ after a ": ": "fake: 2" denies
        # internal/fakepop in internal/loop alone, its files on one line.
        "fake: 1":
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakeimap
        "fake: 2":
          files: [internal/loop/**]
          deny:
            - pkg: example/internal/fakepop
        # Two rules, quoted names that differ after a " #": "fake #2" denies
        # internal/fakedhcp in internal/loop alone, its files on one line.
        "fake #1":
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakedns
        "fake #2":
          files: [internal/loop/**]
          deny:
            - pkg: example/internal/fakedhcp
        # An anchor after the files key.
        fakesnmp-only-in-tests:
          files: &tests-only
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakesnmp
        # A name begun with a "-".
        -fakeldap-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakeldap
        # A quoted name holding a " - ".
        "fakesmtp - only in tests":
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakesmtp
        # Lists at the indent of their key.
        fakeftp-only-in-tests:
          files:
          - "$all"
          - "!$test"
          deny:
          - pkg: example/internal/fakeftp
        # Keys and values in single quotes.
        'fakeudp-only-in-tests':
          'files':
            - '$all'
            - '!$test'
          'deny':
            - 'pkg': 'example/internal/fakeudp'
        # A space before each key's ":".
        fakesctp-only-in-tests :
          files :
            - "$all"
            - "!$test"
          "deny" :
            - pkg : example/internal/fakesctp
        # Keys in capitals, which golangci-lint reads in any case.
        FAKEAMQP-ONLY-IN-TESTS:
          Files:
            - "$all"
            - "!$test"
          DENY:
            - pkg: example/internal/fakeamqp
        # A desc of lines, one of which reads like a deny item for
        # internal/memstore, which the rule denies nothing.
        fakedb-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/fakedb
              desc: |
                a test helper; a test that wants a store of its own imports
                - pkg: example/internal/memstore
END
  # YAML ignores spaces at a line's end, and a line of spaces alone, which an
  # editor would trim from a heredoc here, so they come in through variables.
  local spaces="  " spaces_alone="          "
  cat <<END
        # Spaces after a file.
        fakepager-only-in-tests:
          files:
            - "\$all"
            - "!\$test"${spaces}
          deny:
            - pkg: example/internal/fakepager
        # A line of spaces alone among the files.
        fakeprint-only-in-tests:
          files:
            - "\$all"
${spaces_alone}
            - "!\$test"
          deny:
            - pkg: example/internal/fakeprint
END
}

# alike makes a repository at dir whose layout marks each package named
# (tests only) and tracks a file in each, with the go.mod of module example
# and a .golangci.yml that turns depguard on and holds the alike rules.
#   alike <dir> <package-dir>...
alike() {
  local dir="$1" package marks=() files=()
  shift
  for package in "$@"; do
    marks+=("${package} (tests only)")
    files+=("${package}/x.go")
  done
  repo "${dir}" "${marks[@]}" -- "${files[@]}"
  module "${dir}"
  {
    depguard_rules_head "default: all"
    alike_rules
  } >"${dir}/.golangci.yml"
}

# lint_then makes a repository at dir whose layout marks internal/rlimit
# (tests only), with the go.mod of module example and a .golangci.yml that
# turns depguard on and holds internal/rlimit to tests, then goes on with
# each line given.
#   lint_then <dir> <line>...
lint_then() {
  local dir="$1"
  shift
  rlimit_marked "${dir}"
  lint_turning "${dir}" "default: all"
  printf '%s\n' "$@" >>"${dir}/.golangci.yml"
}

# refuses makes a repository whose .golangci.yml holds internal/rlimit to
# tests, the lines given at its end, and wants the gate to refuse it, naming
# the line given as the fault by its number and its text.
#   refuses <name> <fault> <line>...
refuses() {
  local name="$1" fault="$2" dir="${workdir}/refused-${cases}" number
  shift 2
  lint_then "${dir}" "$@"
  number="$(grep -nFx -- "${fault}" "${dir}/.golangci.yml" | cut -d: -f1)"
  expect_output 2 "${name}" $'(tests only):\n  '"${number}: ${fault#"${fault%%[! ]*}"}"$'\n' \
    run_in "${dir}" "${check}"
}

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  expect_exit "$1" "$2" run_in "$3" "${check}"
}

# expect_unheld runs the gate in a repository and wants it to fail naming the
# one package given, and no other, as held by no rule.
#   expect_unheld <name> <package-dir> <dir>
expect_unheld() {
  expect_output fail "$1" $'tests:\n  '"$2/ (pkg: example/$2)"$'\nGive each' run_in "$3" "${check}"
}

# A comment beside a rule's file or its files key, a line commented out among
# its files at the margin or at its key's indent, a blank line or a line of
# spaces alone among the rules, and spaces after a file are YAML's, not the
# rule's, so each rule holds its package as one without does.
for entry in fakemail:"a comment beside a file" fakechat:"a line commented out at the margin" \
  fakefax:"a line commented out at the files' indent" fakecache:"a blank line above it, a comment beside its files key" \
  fakepager:"spaces after a file" fakeprint:"a line of spaces alone among its files"; do
  alike "${workdir}/${entry%%:*}" "internal/${entry%%:*}"
  expect pass "a marked package whose rule has ${entry#*:}" "${workdir}/${entry%%:*}"
done

# A "#" with no space before it is part of a YAML name, not a comment, and a
# ":" with no space after it is too, as are a ": " or a " #" inside quotes, so
# rules whose names differ only after one are two rules: the first of each
# pair holds its package, and the second's package fails by name.
hashed="${workdir}/hashed"
alike "${hashed}" internal/fakeirc internal/fakexmpp
expect_unheld "marked packages of two rules whose names differ after a \"#\"" internal/fakexmpp "${hashed}"
coloned="${workdir}/coloned"
alike "${coloned}" internal/fakesip internal/fakemqtt
expect_unheld "marked packages of two rules whose names differ after a \":\"" internal/fakemqtt "${coloned}"
quotedcolon="${workdir}/quotedcolon"
alike "${quotedcolon}" internal/fakeimap internal/fakepop
expect_unheld "marked packages of two rules whose quoted names differ after a \": \"" \
  internal/fakepop "${quotedcolon}"
quotedhash="${workdir}/quotedhash"
alike "${quotedhash}" internal/fakedns internal/fakedhcp
expect_unheld "marked packages of two rules whose quoted names differ after a \" #\"" \
  internal/fakedhcp "${quotedhash}"

# A key ends at the ":" a space or the line's end follows, so an anchor after
# it is no part of its name; a "-" begins a list item only where a space
# follows it, and a " - " inside a quoted name begins none; a list may stand
# at its key's indent; quotes may be single; a key may have spaces before its
# ":"; and golangci-lint reads keys in any case. So each of these rules holds
# its package as any other does.
for entry in fakesnmp:"has an anchor after its files key" fakeldap:"has a name begun with a \"-\"" \
  fakesmtp:"has a quoted name holding a \" - \"" fakeftp:"has lists at their key's indent" \
  fakeudp:"has keys and values in single quotes" fakesctp:"has spaces before its keys' \":\"" \
  fakeamqp:"has keys in capitals"; do
  alike "${workdir}/${entry%%:*}" "internal/${entry%%:*}"
  expect pass "a marked package whose rule ${entry#*:}" "${workdir}/${entry%%:*}"
done

# The lines of a desc written as a block are its text, though one reads like
# a deny item, so the rule holds its own package and not the one that line
# names.
block="${workdir}/block"
alike "${block}" internal/fakedb internal/memstore
expect_unheld "a marked package only a line of a block desc names" internal/memstore "${block}"

# golangci-lint reads every key in lower case, and splits each at its "."s
# into the path of keys they name, so two keys that read alike so set one
# setting, and golangci-lint keeps one of them: the gate reads neither, and a
# rule they name holds nothing, though the gate reads the second key's list
# of files, on one line, as no narrowing. A rule beside one whose name
# differs from its in case alone, a rule's files beside its Files, its deny
# beside its Deny, a rule beside one named for its files, and linters'
# settings beside its Settings each leave internal/rlimit unheld; a path of
# keys written as one dotted key, with nothing else to set it, holds it as
# the path does.
lint_then "${workdir}/casenames" "        Rlimit-only-in-tests:" "          files: [internal/loop/**]" \
  "          deny:" "            - pkg: example/internal/other"
expect_unheld "a marked package of a rule beside one whose name differs from its in case alone" \
  internal/rlimit "${workdir}/casenames"
lint_then "${workdir}/casefiles" "          Files: [internal/loop/**]"
expect_unheld "a marked package of a rule with files and Files both" internal/rlimit "${workdir}/casefiles"
lint_then "${workdir}/casedeny" "          Deny:" "            - pkg: example/internal/other"
expect_unheld "a marked package of a rule with deny and Deny both" internal/rlimit "${workdir}/casedeny"
lint_then "${workdir}/dottedfiles" "        rlimit-only-in-tests.files: [internal/loop/**]"
expect_unheld "a marked package of a rule beside one named for its files" internal/rlimit "${workdir}/dottedfiles"
lint_then "${workdir}/twosettings" "  Settings:" "    wsl_v5:" "      allow-whole-block: true"
expect_unheld "a marked package of a rule under settings, beside a Settings" internal/rlimit \
  "${workdir}/twosettings"
lint_file "${workdir}/dottedpath" <<'END'
version: "2"
linters:
  default: all
linters.settings.depguard.rules.rlimit-only-in-tests:
  files:
    - "$all"
    - "!$test"
  deny:
    - pkg: example/internal/rlimit
END
expect pass "a marked package of a rule whose path is one dotted key" "${workdir}/dottedpath"

# golangci-lint reads the first YAML document alone, so a rule in a second
# holds nothing, while a "---" ahead of the first begins it and a "..." ends
# it: rules in the first hold their packages, though the next would not.
lint_file "${workdir}/seconddocument" <<'END'
version: "2"
linters:
  default: all
---
linters:
  settings:
    depguard:
      rules:
        rlimit-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/rlimit
END
expect_unheld "a marked package only a second YAML document's rule holds" internal/rlimit \
  "${workdir}/seconddocument"
{
  printf -- '---\n'
  depguard_rules_head "default: all"
  rules_holding internal/rlimit
  printf '...\n'
  depguard_rules_head
  printf '        rlimit-only-in-tests:\n          files:\n            - "**/internal/loop/**"\n'
} | lint_file "${workdir}/firstdocument"
expect pass "a marked package the first YAML document's rule holds, between a --- and a ..." \
  "${workdir}/firstdocument"

# The gate refuses, naming the line, what it cannot read as golangci-lint
# does: an alias, a tag, an anchor beside a value or a key, a merge key, an
# explicit key, an escape, a quote doubled, a flow mapping, a flow sequence
# holding a quote, an anchor, an alias, a tag or another sequence, or running
# on past its line, a value on the line below its item's "-", and a key that
# is not printable ASCII, which golangci-lint lowers as no awk is sure to.
refuses "an alias" "x-copy: *names" "x-names: &names" "  - depguard" "x-copy: *names"
refuses "a tag" "x-binary: !!binary ZGVwZ3VhcmQ=" "x-binary: !!binary ZGVwZ3VhcmQ="
refuses "an anchor beside a value" "x-name: &name depguard" "x-name: &name depguard"
refuses "an anchor beside a key" "&key x-name: depguard" "&key x-name: depguard"
refuses "a merge key" "  <<:" "x-merged:" "  <<:" "    a: b"
refuses "an explicit key" "? linters.disable" "? linters.disable" ": [depguard]"
refuses "an escape in double quotes" 'x-name: "depg\x75ard"' 'x-name: "depg\x75ard"'
refuses "a quoted key holding an escape" '"x-n\x61me": depguard' '"x-n\x61me": depguard'
refuses "a quote doubled in single quotes" "x-name: 'it''s'" "x-name: 'it''s'"
refuses "a quote doubled in a key in single quotes" "'x-it''s': depguard" "'x-it''s': depguard"
refuses "a flow mapping" "x-names: {a: depguard}" "x-names: {a: depguard}"
refuses "an item whose value stands on the line below" "    depguard" "x-names:" "  -" "    depguard"
refuses "a flow sequence holding a quote" 'x-names: ["depguard"]' 'x-names: ["depguard"]'
refuses "an anchor in a flow sequence" "x-names: [&name depguard]" "x-names: [&name depguard]"
refuses "an alias in a flow sequence" "x-copy: [a, *names]" "x-names: &names" "  - b" "x-copy: [a, *names]"
refuses "a tag in a flow sequence" "x-names: [!!str depguard]" "x-names: [!!str depguard]"
refuses "a flow sequence holding another" "x-names: [a, [depguard]]" "x-names: [a, [depguard]]"
refuses "a flow sequence running on past its line" "x-names: [a," "x-names: [a," "  depguard]"
refuses "a key that is not printable ASCII" "x-namé: depguard" "x-namé: depguard"

# A quoted value can run on past its line however the lines after it are
# indented, and golangci-lint reads them into it, so a deny item in a desc
# that does would hold nothing, and the gate refuses the desc.
lint_file "${workdir}/quotedrunon" <<'END'
version: "2"
linters:
  default: all
  settings:
    depguard:
      rules:
        other-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/other
              desc: "a test helper
            - pkg: example/internal/rlimit
              done"
END
expect_output 2 "a quoted desc running on past its line" $'(tests only):\n  13: desc: "a test helper\n' \
  run_in "${workdir}/quotedrunon" "${check}"

# A plain value runs on onto a line below it indented deeper, which golangci-
# lint reads into it, so a file or a pkg that does is not the one its first
# line says, and the gate refuses the line that runs on, as a file or as an
# item.
lint_file "${workdir}/plainrunon" <<'END'
version: "2"
linters:
  default: all
  settings:
    depguard:
      rules:
        rlimit-only-in-tests:
          files:
            - $all
              /x
            - "!$test"
          deny:
            - pkg: example/internal/rlimit
END
expect_output 2 "a plain file running on past its line" $'(tests only):\n  10: /x\n' \
  run_in "${workdir}/plainrunon" "${check}"
lint_file "${workdir}/itemrunon" <<'END'
version: "2"
linters:
  default: all
  settings:
    depguard:
      rules:
        rlimit-only-in-tests:
          files:
            - "$all"
            - "!$test"
          deny:
            - pkg: example/internal/rlimit
                - other
END
expect_output 2 "a plain pkg running on onto a line that reads as an item" $'(tests only):\n  13: - other\n' \
  run_in "${workdir}/itemrunon" "${check}"

finish_tests
