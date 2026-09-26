#!/usr/bin/env bash
#
# Tests for e2e-server.sh: it builds the fixture at a path that is missing,
# an empty directory or one it made before, and refuses, leaving it exactly
# as it was, a path that is a file, a symbolic link or a directory holding
# files it did not make. A stub stands in for bin/workflow and exits at once,
# so the script returns once the fixture is built.
#
# Usage:
#   scripts/e2e-server_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

# This test's own git must not follow the environment it was started with.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

# The script serves the bin/workflow beside its own directory, so a copy of it
# serves the stub.
readonly root="${workdir}/root"
mkdir -p "${root}/scripts" "${root}/bin"
cp "${here}/e2e-server.sh" "${root}/scripts/"
printf '#!/bin/sh\nexit 0\n' >"${root}/bin/workflow"
chmod +x "${root}/bin/workflow"

readonly -a git=(env -i "PATH=${PATH}" "HOME=${workdir}" GIT_CONFIG_NOSYSTEM=1 git)

failures=0
cases=0

# serve runs the script on a fixture path, keeping its output in <path>.out,
# and prints pass or fail.
serve() {
  local path="$1"
  if "${root}/scripts/e2e-server.sh" "${path}" >"${path}.out" 2>&1; then
    echo "pass"
  else
    echo "fail"
  fi
}

# failed records a failed case, with the script's output.
#   failed <name> <path> <what went wrong>
failed() {
  local name="$1" path="$2" problem="$3"
  echo "FAIL ${name}: ${problem}" >&2
  sed 's/^/  /' "${path}.out" >&2
  failures=$((failures + 1))
}

# snapshot describes a path as it stands: every entry under it, where each
# symbolic link points and what each file holds.
snapshot() {
  local entry
  find "$1" -print | sort | while IFS= read -r entry; do
    if [[ -L "${entry}" ]]; then
      printf 'link %s -> %s\n' "${entry}" "$(readlink "${entry}")"
    elif [[ -f "${entry}" ]]; then
      printf 'file %s %s\n' "${entry}" "$(cksum <"${entry}")"
    else
      printf 'directory %s\n' "${entry}"
    fi
  done
}

# expect_refused wants the script to refuse a path and leave it as it was.
#   expect_refused <name> <path>
expect_refused() {
  local name="$1" path="$2" before got after
  cases=$((cases + 1))
  before="$(snapshot "${path}")"
  got="$(serve "${path}")"
  after="$(snapshot "${path}")"

  if [[ "${got}" != "fail" ]]; then
    failed "${name}" "${path}" "want a refusal, got a fixture"
  elif [[ "${after}" != "${before}" ]]; then
    failed "${name}" "${path}" "the refused path changed"
  fi
}

# fixture_problem prints what the fixture at a path lacks, or nothing when it
# is whole: the mark, a home, a bare origin holding main, and a repository
# checked out on docs/notes, a branch with no upstream beside origin/main, with
# notes.txt untracked.
fixture_problem() {
  local path="$1" repo="$1/repo" pushed branch changes
  pushed="$("${git[@]}" --git-dir "${path}/origin.git" log -1 --format=%s main 2>&1 || true)"
  branch="$("${git[@]}" -C "${repo}" symbolic-ref --short HEAD 2>&1 || true)"
  changes="$("${git[@]}" -C "${repo}" status --porcelain 2>&1 || true)"

  if [[ ! -f "${path}/.workflow-e2e-fixture" || ! -d "${path}/home" ]]; then
    echo "no mark or no home"
  elif [[ "${pushed}" != "chore: start the fixture" ]]; then
    echo "the origin holds no main"
  elif ! "${git[@]}" -C "${repo}" rev-parse --verify --quiet refs/remotes/origin/main >/dev/null; then
    echo "the repository has no origin/main"
  elif [[ "${branch}" != "docs/notes" ]]; then
    echo "the repository is not on docs/notes"
  elif "${git[@]}" -C "${repo}" rev-parse --verify --quiet '@{upstream}' >/dev/null 2>&1; then
    echo "docs/notes has an upstream"
  elif [[ "${changes}" != "?? notes.txt" ]]; then
    echo "notes.txt is not the one untracked change"
  fi
}

# expect_fixture wants the script to build the whole fixture at a path.
#   expect_fixture <name> <path>
expect_fixture() {
  local name="$1" path="$2" got problem
  cases=$((cases + 1))
  got="$(serve "${path}")"

  if [[ "${got}" != "pass" ]]; then
    failed "${name}" "${path}" "want a fixture, got a refusal"
    return
  fi

  problem="$(fixture_problem "${path}")"
  if [[ -n "${problem}" ]]; then
    failed "${name}" "${path}" "${problem}"
  fi
}

file="${workdir}/file"
printf 'not a fixture\n' >"${file}"
expect_refused "a file" "${file}"

link="${workdir}/link"
printf 'not a fixture either\n' >"${workdir}/target"
ln -s "${workdir}/target" "${link}"
expect_refused "a symbolic link to a file" "${link}"

foreign="${workdir}/foreign"
mkdir -p "${foreign}"
printf 'kept\n' >"${foreign}/kept.txt"
expect_refused "a directory holding files the script did not make" "${foreign}"

expect_fixture "a missing path" "${workdir}/missing"

empty="${workdir}/empty"
mkdir -p "${empty}"
expect_fixture "an empty directory" "${empty}"

rebuilt="${workdir}/rebuilt"
expect_fixture "a first build" "${rebuilt}"
printf 'stale\n' >"${rebuilt}/stale.txt"
expect_fixture "a directory the script made before" "${rebuilt}"
if [[ -e "${rebuilt}/stale.txt" ]]; then
  echo "FAIL a directory the script made before: stale.txt survived the rebuild" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  echo "e2e-server_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "e2e-server_test: ${cases} case(s) passed."
