#!/usr/bin/env bash
#
# Fail unless CLAUDE.md's layout block names every Go package directory, names
# no directory that holds nothing, and marks "(tests only)" only a package a
# depguard rule holds to tests.
#
# Usage:
#   scripts/check-layout.sh
#
# The layout is how a session finds where a concern lives — where a credential
# is kept, which package talks to which service — so a package left out of it
# is invisible to exactly the reader it was written for. It drifted once: two
# packages, one of them holding the Slack user token, were missing from it.
# This holds both directions. Every directory holding a tracked Go file (but
# one under testdata, which Go never builds) must have a line of its own,
# `internal/messaging/directory/` as well as `internal/messaging/`; and every
# line must name a directory that holds a tracked file, so a package removed
# takes its line with it.
#
# A line that ends "(tests only)" marks a helper only tests import, and
# coverage-gate.sh leaves each such package out of the statement total. So the
# mark has to be the build's word and not the layout's alone: a product package
# marked by mistake would leave the total unseen. Each marked package must be
# one a depguard rule in .golangci.yml denies to every file but a test, a rule
# whose files leave out "$test" and nothing else, whose deny list names the
# package's import path (go.mod's module, then the directory), and whose
# list-mode, if set, is original, where a deny item wins over every allow
# item, as rlimit-only-in-tests does. The rules are read only when a line is
# marked.
#
# The gate reads .golangci.yml as golangci-lint does, as far as it can be sure
# of that, and no further. golangci-lint looks for a .golangci.json,
# .golangci.toml and .golangci.yaml first, tracked or not, and reads the first
# it finds in place of .golangci.yml, so the gate refuses beside one. It reads
# the first YAML document alone, in block style: a key or a list item to a
# line, each key plain or in quotes, and each value plain, in quotes, a flow
# sequence of plain names, or a mapping or a list on the lines below. A block
# scalar, after a "|" or a ">", it passes over unread: as a rule's files, or
# a file among them, it leaves the rule holding nothing, and as a pkg it
# names none. It reads each key in lower case and split at each "." into the
# path of keys it names, as golangci-lint's configuration library does, so
# two keys that read alike so set one setting, of which golangci-lint keeps
# one: the gate then reads neither, so a rule they name holds nothing. It
# refuses, naming the line, anything else it meets (an alias, a tag, a merge
# or explicit key, an escape, a value that runs on past its line) rather than
# read it another way.
#
#   TRACKED FILES ONLY, like the file-length gate: `git ls-files` measures what
#   is committed or staged, so scratch files cannot fail it and a new package
#   counts the moment it is `git add`ed.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/layout.sh
source "${here}/lib/layout.sh"

if (($# > 0)); then
  sed -n '3,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

readonly guide="CLAUDE.md"
readonly lint_config=".golangci.yml"
# golangci-lint's configuration library looks for each of these before it
# looks for lint_config, and reads the first it finds.
readonly configs_read_first=(.golangci.json .golangci.toml .golangci.yaml)

# yaml_lines is the awk each reader of the lint configuration below begins
# with. It reads each line of the first YAML document but a comment or a
# blank, leaving in depth its indent, in item whether it is a list item, in
# keyed and key whether it is or holds a key and that key's name, in kind and
# value the value beside it, and in keys and path the keys it stands under, a
# list item at its key's indent standing under that key. Each path is the
# keys down to its own, lowered and split at each "." as golangci-lint reads
# them, joined by newlines, and a path two keys spell is doubled. It passes
# over a block scalar's lines, and refuses, keeping the first, each line it
# cannot read as golangci-lint would: one that is no key and no item, one
# whose key or value key_line or read_value does not read, and one deeper
# than a line whose value it would run on.
# shellcheck disable=SC2016 # awk's $0, which the shell must leave alone
readonly yaml_lines='
  function refuse() {
    if (refused == "") refused = NR ": " text
  }
  # plain(s) is 0 when s begins as an anchor, an alias, a tag, a flow
  # collection, a quoted scalar or a merge key does, and 1 otherwise.
  function plain(s) {
    return s !~ /^([[{&*!"\047]|<<)/
  }
  # key_line(s) reads s as a key and what follows its ":": a key in quotes
  # holding none of its own nor a backslash, or a plain one ending at the
  # first ":" a space or the line end follows, less spaces before that ":".
  # It sets key and rest and returns 1, or sets key empty and returns 0
  # when s holds no key.
  function key_line(s) {
    key = ""
    if (match(s, /^("[^"\\]*"|\047[^\047]*\047)[[:space:]]*:([[:space:]]|$)/)) {
      key = substr(s, 2, index(substr(s, 2), substr(s, 1, 1)) - 1)
      rest = substr(s, RLENGTH + 1)
      return 1
    }
    if (!plain(s) || !match(s, /:([[:space:]]|$)/)) return 0
    key = substr(s, 1, RSTART - 1)
    rest = substr(s, RSTART + 1)
    sub(/[[:space:]]+$/, "", key)
    return key !~ /[[:space:]]#/
  }
  # read_value(s) reads s, the text after a key or a list item, as a value
  # and the comment beside it: none, or an anchor alone, leaving the value to
  # the lines below; a block scalar, whose text is the lines below and whose
  # value it leaves empty; a flow sequence of plain names, none an anchor,
  # alias or tag, kept as written; a scalar in quotes holding none of its own
  # nor a backslash, less its quotes; or a plain scalar. It sets kind and
  # value, and refuses anything else.
  function read_value(s) {
    value = ""
    kind = "scalar"
    sub(/^[[:space:]]+/, "", s)
    if (s ~ /^(&[^[:space:]]*)?([[:space:]]*#.*)?$/) kind = "none"
    else if (match(s, /^[|>][-+0-9]*/)) kind = "block"
    else if (match(s, /^("[^"\\]*"|\047[^\047]*\047)/)) value = substr(s, 2, RLENGTH - 2)
    else if (match(s, /^\[[^][{}#"\047]*\]/)) {
      kind = "flow"
      value = substr(s, 1, RLENGTH)
      if (value ~ /[[,][[:space:]]*[&*!]/) refuse()
    } else if (plain(s)) {
      value = s
      sub(/[[:space:]]+#.*$/, "", value)
      return
    } else {
      refuse()
      return
    }
    if (kind != "none" && substr(s, RLENGTH + 1) !~ /^([[:space:]]+#.*)?$/) refuse()
  }
  /^[[:space:]]*(#|$)/ { next }
  /^(---|\.\.\.)([[:space:]]|$)/ {
    if (begun) exit
    begun = 1
    next
  }
  {
    begun = 1
    match($0, /^ */)
    depth = RLENGTH
    if (block && depth > owner) next
    text = substr($0, depth + 1)
    sub(/[[:space:]]+$/, "", text)
    if (valued && depth > owner) refuse()
    item = text ~ /^-([[:space:]]|$)/
    body = text
    if (item) {
      body = substr(text, 2)
      sub(/^[[:space:]]+/, "", body)
    }
    keyed = key_line(body)
    if (!keyed && !item || keyed && key ~ /[^ -~]/) refuse()
    read_value(keyed ? rest : body)
    owner = keyed ? depth + length(text) - length(body) : depth
    block = kind == "block"
    valued = kind == "scalar"
    while (keys > 0 && (indent[keys] > depth || indent[keys] == depth && !item)) keys--
    if (!item) {
      name = tolower(key)
      gsub(/\./, "\n", name)
      path[keys + 1] = keys ? path[keys] "\n" name : name
      indent[++keys] = depth
      if (seen[path[keys]]++) doubled[path[keys]] = 1
    }
  }
'

# unread_line prints the first line of the lint configuration yaml_lines
# refuses, after its number and a ":", or nothing when it refuses none.
#   unread_line <lint-config>
unread_line() {
  awk "${yaml_lines}"'
    END { if (refused != "") print refused }
  ' "$1"
}

# held_to_tests prints, one a line, each package a depguard rule in the lint
# configuration denies to every file but a test: a rule under
# linters.settings.depguard.rules whose files leave out "$test" and nothing
# else, "!$test" with or without "$all" (depguard reads a list of negations
# alone as every file, less those), whose deny list names the package, and
# whose list-mode, if set, is original in any letter case (lax and strict let
# an allow item longer than the deny item through), where no two keys spell
# the rule, its files, its deny list or a key above them. It reads only a
# rule's list-mode, the list items under its files and deny keys, and a deny
# item's pkg only from the item's own line.
#   held_to_tests <lint-config>
held_to_tests() {
  awk "${yaml_lines}"'
    BEGIN { rules = "linters\nsettings\ndepguard\nrules\n" }
    function spelled_once(spelling,   count, i, part, prefix) {
      count = split(spelling, part, "\n")
      prefix = part[1]
      for (i = 1; i <= count; i++) {
        if (i > 1) prefix = prefix "\n" part[i]
        if (prefix in doubled) return 0
      }
      return 1
    }
    item && index(path[keys], rules) == 1 && split(substr(path[keys], length(rules) + 1), part, "\n") == 2 {
      rule = rules part[1]
      if (part[2] == "files" && value == "!$test") leaves_out_tests[rule] = 1
      else if (part[2] == "files" && value != "$all") narrowed[rule] = 1
      else if (part[2] == "deny" && key == "pkg") denies[rule] = denies[rule] value "\n"
    }
    !item && index(path[keys], rules) == 1 && split(substr(path[keys], length(rules) + 1), part, "\n") == 2 && part[2] == "list-mode" && tolower(value) != "original" {
      allow_can_win[rules part[1]] = 1
    }
    END {
      for (rule in denies) {
        if (leaves_out_tests[rule] && !narrowed[rule] && !allow_can_win[rule] && spelled_once(rule "\nfiles") && spelled_once(rule "\ndeny")) printf "%s", denies[rule]
      }
    }
  ' "$1"
}

# Capture the whole Go file list up front: a git that cannot answer must stop
# the gate, not let its failure escape set -e and leave it comparing nothing.
if ! go_files="$(git ls-files '*.go')"; then
  echo "check-layout: could not list tracked files with git." >&2
  exit 2
fi

if [[ ! -f "${guide}" ]]; then
  echo "check-layout: no ${guide} here to read the layout from." >&2
  exit 2
fi

named="$(layout_dirs "${guide}" | sort -u)"

if [[ -z "${named}" ]]; then
  echo "check-layout: ${guide} has no layout block (a \`\`\`text block after 'Layout:') to compare with." >&2
  exit 2
fi

marked="$(layout_tests_only_dirs "${guide}")"
unheld=""
if [[ -n "${marked}" ]]; then
  module="$(awk '$1 == "module" { print $2; exit }' go.mod 2>/dev/null || true)"
  if [[ -z "${module}" ]]; then
    echo "check-layout: no module path in a go.mod here to find the depguard rule of a package ${guide} marks (tests only) by." >&2
    exit 2
  fi
  for config in "${configs_read_first[@]}"; do
    if [[ -f "${config}" ]]; then
      echo "check-layout: golangci-lint reads ${config} ahead of ${lint_config}, the one file the gate reads, so it cannot tell whether depguard holds the packages ${guide}'s layout marks (tests only)." >&2
      echo "Keep golangci-lint's configuration in ${lint_config} alone, or drop the marks: coverage-gate.sh leaves a marked package out of the statement total." >&2
      exit 2
    fi
  done
  held=""
  if [[ -f "${lint_config}" ]]; then
    unread="$(unread_line "${lint_config}")"
    if [[ -n "${unread}" ]]; then
      echo "check-layout: ${lint_config} holds a line the gate cannot read as golangci-lint does, so it cannot tell whether depguard holds the packages ${guide}'s layout marks (tests only):" >&2
      echo "  ${unread}" >&2
      echo "Write it as the rest is, a key or a list item to a line in block style with no alias, tag, merge or explicit key, escape, or value running on past its line, or drop the marks: coverage-gate.sh leaves a marked package out of the statement total." >&2
      exit 2
    fi
    held="$(held_to_tests "${lint_config}")"
  fi
  while IFS= read -r dir; do
    if ! grep -Fqx -- "${module}/${dir}" <<<"${held}"; then
      unheld+="${dir}"$'\n'
    fi
  done <<<"${marked}"
  unheld="${unheld%$'\n'}"
fi

packages="$(printf '%s\n' "${go_files}" | grep -vE '(^|/)testdata/' | sed -n 's|/[^/]*$||p' | sort -u || true)"

missing="$(comm -23 <(printf '%s\n' "${packages}") <(printf '%s\n' "${named}") | sed '/^$/d')"

stale=""
while IFS= read -r dir; do
  if [[ -z "$(git ls-files -- "${dir}" | head -n 1)" ]]; then
    stale+="${dir}"$'\n'
  fi
done <<<"${named}"
stale="${stale%$'\n'}"

# list_dirs prints each directory of a newline-separated list, indented, with
# the trailing slash the layout writes.
#   list_dirs <dirs>
list_dirs() {
  local dir
  while IFS= read -r dir; do
    printf '  %s/\n' "${dir}"
  done <<<"$1"
}

if [[ -n "${missing}" ]]; then
  echo "check-layout: Go packages ${guide}'s layout does not name:" >&2
  list_dirs "${missing}" >&2
  echo "Add a line for each, saying what it holds." >&2
fi

if [[ -n "${stale}" ]]; then
  echo "check-layout: directories ${guide}'s layout names that hold no tracked file:" >&2
  list_dirs "${stale}" >&2
  echo "Remove each line, or move it to where the directory went." >&2
fi

if [[ -n "${unheld}" ]]; then
  echo "check-layout: packages ${guide}'s layout marks (tests only) that no depguard rule holds to tests:" >&2
  while IFS= read -r dir; do
    printf '  %s/ (pkg: %s/%s)\n' "${dir}" "${module}" "${dir}" >&2
  done <<<"${unheld}"
  echo "Give each a depguard rule in ${lint_config} that denies its pkg to the files \"\$all\" and \"!\$test\" alone, as rlimit-only-in-tests does, or drop its mark: coverage-gate.sh leaves a marked package out of the statement total." >&2
fi

if [[ -n "${missing}" || -n "${stale}" || -n "${unheld}" ]]; then
  exit 1
fi

echo "check-layout: ${guide}'s layout names every Go package, and only directories that hold files; depguard holds each package it marks (tests only) to tests."
