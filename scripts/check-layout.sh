#!/usr/bin/env bash
#
# Fail unless CLAUDE.md's layout block names every Go package directory, names
# no directory that holds nothing, and marks "(tests only)" only a package a
# depguard rule holds to tests, with depguard on, and every package of the
# repository such a rule holds.
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
# item, as rlimit-only-in-tests does. Its files may also leave out a
# directory, "!**/<dir>/**" with no pattern in <dir>, when the directory
# holds a package a rule holds to tests in turn, as is every package under a
# directory of that name at any depth, whose files the pattern leaves out
# too: a test helper may import another, since only tests reach either.
#
# The other way round, a package of the repository a rule holds to tests must
# be marked, so the marks say what the rules do and no edit to CLAUDE.md alone
# can change what coverage-gate.sh counts: such an edit is docs only, and CI
# runs no coverage for one. With no line marked the rules are still read, when
# .golangci.yml is there and the gate can read it as golangci-lint does, so
# taking every mark off fails as taking one off does.
#
# A rule holds only while depguard runs, so a mark also fails when
# .golangci.yml does not make sure it does: when linters.disable lists
# depguard, when linters.default is not all and linters.enable does not list
# it, or when linters holds a key the gate does not read, or a default,
# enable or disable it does not read, as a block scalar. Of golangci-lint's
# defaults only all is sure to run depguard: standard, the default when none
# is set, and none leave it out, and fast holds it only while golangci-lint
# counts depguard fast, which the configuration does not say. Whether depguard
# runs is read only when a line is marked.
# Exclusion rules and //nolint:depguard stay unread, the maintainer's choice,
# and so do issues.new and new-from-rev, -patch and -merge-base, which hide
# what depguard finds as they do, and run.issues-exit-code, which set to 0
# lets golangci-lint pass with what depguard finds printed.
#
# The gate reads .golangci.yml as golangci-lint does, as far as it can be sure
# of that, and no further. golangci-lint looks for a .golangci.json,
# .golangci.toml and .golangci.yaml first, tracked or not, and reads the first
# it finds in place of .golangci.yml, so the gate refuses beside one. It reads
# the first YAML document alone, in block style: a key or a list item to a
# line, each key plain or in quotes, and each value plain, in quotes, a flow
# sequence of plain names, or a mapping or a list on the lines below. A block
# scalar, after a "|" or a ">", it passes over unread: as a rule's files, or
# a file among them, it leaves the rule holding nothing, as a pkg it names
# none, and as linters' default, enable or disable, or an item of either
# list, it fails the mark. It reads each key in lower case and split at each
# "." into the path of keys it names, as golangci-lint's configuration
# library does, so two keys that read alike so set one setting, of which
# golangci-lint keeps one: the gate then reads neither, so a rule they name
# holds nothing and a linters key they set fails the mark. It refuses, naming
# the line, anything else it meets (an alias, a tag, a merge or explicit key,
# an escape, a value that runs on past its line) rather than read it another
# way.
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

# denials prints, one a line, each package a depguard rule in the lint
# configuration denies to every file but a test and the directories the line
# names after it, each after a tab: a rule under
# linters.settings.depguard.rules whose files leave out "$test", "!$test"
# with or without "$all" (depguard reads a list of negations alone as every
# file, less those), and besides it only directories, each as "!**/<dir>/**"
# with no pattern in <dir>, whose deny list names the package, and whose
# list-mode, if set, is original in any letter case (lax and strict let an
# allow item longer than the deny item through), where no two keys spell the
# rule, its files, its deny list or a key above them. It reads only a rule's
# list-mode, the list items under its files and deny keys, and a deny item's
# pkg only from the item's own line.
#   denials <lint-config>
denials() {
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
      else if (part[2] == "files" && value ~ /^!\*\*\/[A-Za-z0-9_][A-Za-z0-9_.-]*(\/[A-Za-z0-9_][A-Za-z0-9_.-]*)*\/\*\*$/) {
        leaves_out[rule] = leaves_out[rule] "\t" substr(value, 5, length(value) - 7)
      } else if (part[2] == "files" && value != "$all") narrowed[rule] = 1
      else if (part[2] == "deny" && key == "pkg") denies[rule] = denies[rule] value "\n"
    }
    !item && index(path[keys], rules) == 1 && split(substr(path[keys], length(rules) + 1), part, "\n") == 2 && part[2] == "list-mode" && tolower(value) != "original" {
      allow_can_win[rules part[1]] = 1
    }
    END {
      for (rule in denies) {
        if (!leaves_out_tests[rule] || narrowed[rule] || allow_can_win[rule] || !spelled_once(rule "\nfiles") || !spelled_once(rule "\ndeny")) continue
        count = split(denies[rule], denied, "\n")
        for (i = 1; i < count; i++) print denied[i] leaves_out[rule]
      }
    }
  ' "$1"
}

# held_to_tests prints, one a line, each package one of the denials holds to
# tests: a denial that leaves out no directory, or one each directory of which
# is a package held to tests, as is every package under a directory of that
# name, at any depth, since "**/<dir>/**" leaves out the files of each.
# A package held through another is held once that one is, so a chain holds
# and a cycle holds none of its packages.
#   held_to_tests <denials> <module> <packages>
held_to_tests() {
  local denials="$1" module="$2" packages="$3" held="" grew=yes line blocking
  local -a denial
  while [[ -n "${grew}" ]]; do
    grew=""
    while IFS= read -r line; do
      [[ -n "${line}" ]] || continue
      IFS=$'\t' read -r -a denial <<<"${line}"
      grep -Fqx -- "${denial[0]}" <<<"${held}" && continue
      blocking="$(unheld_left_out "${held}" "${module}" "${packages}" "${denial[@]:1}")"
      [[ -z "${blocking}" ]] || continue
      held+="${denial[0]}"$'\n'
      grew=yes
    done <<<"${denials}"
  done
  printf '%s' "${held}"
}

# unheld_left_out prints the first directory given that is not a package
# held to tests, or that has a package not held under it or under any
# directory of that name at any depth; nothing when every one is held.
#   unheld_left_out <held> <module> <packages> [<dir>...]
unheld_left_out() {
  local held="$1" module="$2" packages="$3" dir package
  shift 3
  for dir in "$@"; do
    if ! grep -Fqx -- "${module}/${dir}" <<<"${held}"; then
      printf '%s\n' "${dir}"
      return
    fi
    while IFS= read -r package; do
      if [[ "/${package}/" == *"/${dir}/"* ]] && ! grep -Fqx -- "${module}/${package}" <<<"${held}"; then
        printf '%s\n' "${dir}"
        return
      fi
    done <<<"${packages}"
  done
}

# depguard_off prints why the lint configuration does not make sure depguard
# runs, and on the line after it the change that answers that reason, or
# nothing when it does: golangci-lint runs it when linters.default is all or
# linters.enable lists it, and linters.disable does not. It reads only
# the keys directly under linters, so a default of a linter's settings, as
# wsl_v5 has, or of another top-level key, as severity has, says nothing here,
# and it reads none of them but default, enable, disable, settings and
# exclusions. It finds depguard in enable or disable as one of the list's
# items, on lines of their own, or on the key's line in a flow sequence or
# in a string, which golangci-lint splits at its commas. A block scalar as
# default, enable or disable, or as an item of either list, it does not read,
# and says so: golangci-lint reads the block's text as the setting, a list's
# split at its commas too.
#   depguard_off <lint-config>
depguard_off() {
  awk "${yaml_lines}"'
    function lists(setting, names,   count, i, name) {
      gsub(/^\[[[:space:]]*|[[:space:]]*\]$/, "", names)
      count = split(names, name, /[[:space:]]*,[[:space:]]*/)
      for (i = 1; i <= count; i++) if (name[i] == "depguard") listed[setting] = 1
    }
    kind == "block" && (path[keys] == "linters\ndefault" || path[keys] == "linters\nenable" || path[keys] == "linters\ndisable") {
      block_setting = "linters." substr(path[keys], length("linters\n") + 1)
      unread = block_setting " holds a block scalar"
      unread_change = "Write " block_setting " without a block scalar"
    }
    !item && index(path[keys], "linters\n") == 1 {
      split(path[keys], part, "\n")
      if (part[2] !~ /^(default|enable|disable|settings|exclusions)$/) {
        unread = "linters." part[2] " is set"
        unread_change = "Leave linters." part[2] " unset"
      }
      if (path[keys] == "linters\ndefault") linters_default = value
      if (path[keys] == "linters\nenable" || path[keys] == "linters\ndisable") lists(part[2], value)
    }
    item && (path[keys] == "linters\nenable" || path[keys] == "linters\ndisable") && value == "depguard" {
      listed[substr(path[keys], length("linters\n") + 1)] = 1
    }
    END {
      count = split("linters,linters\ndefault,linters\nenable,linters\ndisable", spelling, ",")
      for (i = 1; i <= count; i++) {
        if (!(spelling[i] in doubled)) continue
        gsub(/\n/, ".", spelling[i])
        print spelling[i] " is set by two keys golangci-lint reads alike, and it keeps one of them"
        print "Set " spelling[i] " by one key alone"
        exit
      }
      if (unread != "") {
        print unread ", which the gate does not read"
        print unread_change
      } else if (listed["disable"]) {
        print "linters.disable lists depguard"
        print "Take depguard off linters.disable"
      } else if (linters_default != "all" && !listed["enable"]) {
        if (linters_default == "") linters_default = "unset, so standard"
        print "linters.default is " linters_default ", and linters.enable does not list depguard"
        print "List depguard under linters.enable"
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

packages="$(printf '%s\n' "${go_files}" | grep -vE '(^|/)testdata/' | sed -n 's|/[^/]*$||p' | sort -u || true)"

marked="$(layout_tests_only_dirs "${guide}")"
module="$(awk '$1 == "module" { print $2; exit }' go.mod 2>/dev/null || true)"
held=""
unheld=""
depguard_off_because=""
depguard_on_by=""
if [[ -n "${marked}" ]]; then
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
  depguard_off_because="there is no ${lint_config}, so linters.default is standard"
  depguard_on_by="Add a ${lint_config} whose linters.enable lists depguard"
  if [[ -f "${lint_config}" ]]; then
    unread="$(unread_line "${lint_config}")"
    if [[ -n "${unread}" ]]; then
      echo "check-layout: ${lint_config} holds a line the gate cannot read as golangci-lint does, so it cannot tell whether depguard holds the packages ${guide}'s layout marks (tests only):" >&2
      echo "  ${unread}" >&2
      echo "Write it as the rest is, a key or a list item to a line in block style with no alias, tag, merge or explicit key, escape, or value running on past its line, or drop the marks: coverage-gate.sh leaves a marked package out of the statement total." >&2
      exit 2
    fi
    # Sorted, since awk's for-in prints the denials in no set order: in one
    # order on every awk, the depguard test can hold a package only on a
    # second pass, and so test that a chain holds.
    held="$(held_to_tests "$(denials "${lint_config}" | LC_ALL=C sort)" "${module}" "${packages}")"
    off_and_change="$(depguard_off "${lint_config}")"
    depguard_off_because="${off_and_change%%$'\n'*}"
    depguard_on_by="${off_and_change#*$'\n'}"
  fi
  while IFS= read -r dir; do
    if ! grep -Fqx -- "${module}/${dir}" <<<"${held}"; then
      unheld+="${dir}"$'\n'
    fi
  done <<<"${marked}"
  unheld="${unheld%$'\n'}"
elif [[ -n "${module}" && -f "${lint_config}" ]]; then
  unread="$(unread_line "${lint_config}")"
  if [[ -z "${unread}" ]]; then
    held="$(held_to_tests "$(denials "${lint_config}" | LC_ALL=C sort)" "${module}" "${packages}")"
  fi
fi

unmarked=""
while IFS= read -r package; do
  dir="${package#"${module}/"}"
  [[ -n "${package}" && "${dir}" != "${package}" ]] || continue
  if grep -Fqx -- "${dir}" <<<"${packages}" && ! grep -Fqx -- "${dir}" <<<"${marked}"; then
    unmarked+="${dir}"$'\n'
  fi
done <<<"${held}"
unmarked="${unmarked%$'\n'}"

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
  echo "Give each a depguard rule in ${lint_config} that denies its pkg to the files \"\$all\" and \"!\$test\" alone, as rlimit-only-in-tests does, or besides them leaves out only a \"!**/<dir>/**\" whose every package such a rule holds, or drop its mark: coverage-gate.sh leaves a marked package out of the statement total." >&2
fi

if [[ -n "${unmarked}" ]]; then
  echo "check-layout: packages a depguard rule in ${lint_config} holds to tests that ${guide}'s layout does not mark (tests only):" >&2
  while IFS= read -r dir; do
    printf '  %s/ (pkg: %s/%s)\n' "${dir}" "${module}" "${dir}" >&2
  done <<<"${unmarked}"
  echo "Mark each line (tests only), or drop its rule: coverage-gate.sh reads the marks, so they must say what the rules do." >&2
fi

if [[ -n "${depguard_off_because}" ]]; then
  echo "check-layout: a depguard rule holds the packages ${guide}'s layout marks (tests only) to tests only while depguard runs, and ${lint_config} does not make sure it does: ${depguard_off_because}." >&2
  echo "${depguard_on_by}, or drop the marks: coverage-gate.sh leaves a marked package out of the statement total." >&2
fi

if [[ -n "${missing}" || -n "${stale}" || -n "${unheld}" || -n "${unmarked}" || -n "${depguard_off_because}" ]]; then
  exit 1
fi

echo "check-layout: ${guide}'s layout names every Go package, and only directories that hold files; depguard holds each package it marks (tests only) to tests, and it marks each one depguard holds."
