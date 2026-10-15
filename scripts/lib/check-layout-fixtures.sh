# shellcheck shell=bash
#
# The fixtures check-layout.sh's three tests build their repositories from: a
# CLAUDE.md whose layout block names the directories a case gives, a
# repository holding it and the files the case tracks, and the go.mod and
# depguard rules the gate looks a package marked "(tests only)" up in.
# check-layout_test.sh drives the layout block against the tree and the mark's
# need of a rule; check-layout_depguard_test.sh which rules hold a package;
# and check-layout_yaml_test.sh how the gate reads .golangci.yml's YAML, and
# what it refuses to.
#
# Sourced, never run, after lib/testing.sh:
#
#   # shellcheck source=lib/check-layout-fixtures.sh
#   source "${here}/lib/check-layout-fixtures.sh"

# layout writes a CLAUDE.md whose layout block names each argument, as the real
# one does: the directory with a trailing slash, then what it holds, then the
# mark an argument carries after its directory, as "internal/rlimit (tests
# only)" does.
#   layout <dir>[ <mark>]...
layout() {
  cat <<'END'
# CLAUDE.md

Layout:

```text
END
  local entry dir
  for entry in "$@"; do
    dir="${entry%% *}"
    printf '%-22s what it holds%s\n' "${dir}/" "${entry#"${dir}"}"
  done
  cat <<'END'
```

## Common commands

```text
not/a/directory/   in another block
```
END
}

# repo makes a git repository at dir with a CLAUDE.md naming the directories
# after the first `--`, and a tracked file at each path after it.
#   repo <dir> <named-dir>... -- <tracked-path>...
repo() {
  local dir="$1"
  shift
  local listed=()
  while [[ "$1" != "--" ]]; do
    listed+=("$1")
    shift
  done
  shift

  mkdir -p "${dir}"
  git -C "${dir}" init -q
  layout "${listed[@]}" >"${dir}/CLAUDE.md"
  git -C "${dir}" add CLAUDE.md

  local path
  for path in "$@"; do
    mkdir -p "${dir}/$(dirname "${path}")"
    printf 'package x\n' >"${dir}/${path}"
    git -C "${dir}" add "${path}"
  done
}

# module writes, in a repository repo made, the go.mod of a module named
# example, by which the gate names a marked package's import path.
#   module <dir>
module() {
  printf 'module example\n\ngo 1.27\n' >"$1/go.mod"
}

# depguard_rules_head prints a .golangci.yml down to the key depguard's rules
# stand under, linters.settings.depguard.rules, each rule to follow it at the
# indent the root's own rules stand at. Each line given stands under linters
# ahead of its settings, as the root's "default: all", which turns depguard
# on, does.
#   depguard_rules_head [<linters-line>...]
depguard_rules_head() {
  printf 'version: "2"\n\nlinters:\n'
  if (($# > 0)); then
    printf '  %s\n' "$@"
  fi
  printf '  settings:\n    depguard:\n      rules:\n'
}

# rules_holding prints a depguard rule for each package named that holds it to
# tests, as the root's *-only-in-tests rules do: its files "$all" and
# "!$test", its deny list the package's import path.
#   rules_holding <package-dir>...
rules_holding() {
  local package
  for package in "$@"; do
    cat <<END
        # $(basename "${package}") is a test helper.
        $(basename "${package}")-only-in-tests:
          files:
            - "\$all"
            - "!\$test"
          deny:
            - pkg: example/${package}
              desc: a test helper; use it only from a _test.go file
END
  done
}

# rlimit_marked makes a repository at dir whose layout marks internal/rlimit
# (tests only), and which tracks a file in it.
#   rlimit_marked <dir>
rlimit_marked() {
  repo "$1" "internal/rlimit (tests only)" -- internal/rlimit/rlimit.go
}

# lint_turning writes, in a repository repo made, the go.mod of a module
# named example and a .golangci.yml whose depguard rules hold internal/rlimit
# to tests, each line given standing under its linters ahead of its settings.
#   lint_turning <dir> [<linters-line>...]
lint_turning() {
  local dir="$1"
  shift
  module "${dir}"
  {
    depguard_rules_head "$@"
    rules_holding internal/rlimit
  } >"${dir}/.golangci.yml"
}

# lint_file makes a repository at dir whose layout marks internal/rlimit
# (tests only), with the go.mod of module example and the .golangci.yml read
# from standard input.
#   lint_file <dir> <<'END' ... END
lint_file() {
  rlimit_marked "$1"
  module "$1"
  cat >"$1/.golangci.yml"
}
