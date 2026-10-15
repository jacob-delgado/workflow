# shellcheck shell=bash
# shellcheck disable=SC2154 # here and workdir are the sourcing test's
#
# The fixtures check-tech-debt-paths.sh's tests share: a repository holding a
# given TECH_DEBT.md, and the gate run in one. check-tech-debt-paths_test.sh
# tests what reads as a repository path and how one is looked up, and
# check-tech-debt-paths_markdown_test.sh how the Markdown around a path is
# read.
#
# Sourced, never run, after lib/testing.sh, which sets ${workdir}; check is
# the gate beside the sourcing test's ${here}:
#
#   # shellcheck source=lib/tech-debt-paths-fixtures.sh
#   source "${here}/lib/tech-debt-paths-fixtures.sh"

readonly check="${here}/check-tech-debt-paths.sh"

# repo makes a repository tracking internal/tui/model.go, web/src/app.tsx,
# scripts/gate.sh and a TECH_DEBT.md holding the text on standard input.
#   repo <name> <<'EOF' ... EOF
repo() {
  local dir="${workdir}/$1"
  mkdir -p "${dir}/internal/tui" "${dir}/web/src" "${dir}/scripts"
  git -C "${dir}" init -q
  touch "${dir}/internal/tui/model.go" "${dir}/web/src/app.tsx" "${dir}/scripts/gate.sh"
  cat >"${dir}/TECH_DEBT.md"
  git -C "${dir}" add -A
  printf '%s' "${dir}"
}

# gate runs the check in a repository.
#   gate <dir>
gate() {
  run_in "$1" "${check}"
}

# gate_under runs the check in a repository with the given variables set.
#   gate_under <dir> <name=value>...
gate_under() {
  local dir="$1"
  shift
  (cd "${dir}" && env "$@" "${check}")
}

# expect_backlog runs the gate over a fresh repository whose TECH_DEBT.md holds
# standard input, and wants the answer expect_output wants.
#   expect_backlog <want> <name> <text> <<'EOF' ... EOF
expect_backlog() {
  local dir
  backlogs=$((backlogs + 1))
  dir="$(repo "backlog-${backlogs}")"
  expect_output "$1" "$2" "$3" gate "${dir}"
}
backlogs=0
