#!/usr/bin/env bash
#
# Tests for e2e-server.sh: it builds the fixture at a path that is missing,
# an empty directory or one it made before, and serves it on the port it is
# given; it refuses, leaving it exactly as it was, a path that is a file, a
# symbolic link or a directory holding files it did not make; and it stops,
# making nothing, when someone else makes the path again as it is cleared. A
# stub stands in for bin/workflow, writes down the arguments it was run with
# and exits at once, so the script returns once the fixture is built.
#
# Usage:
#   scripts/e2e-server_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly here

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The script serves the bin/workflow beside its own directory, so a copy of it
# serves the stub, which says where it serves on stderr as the server does.
readonly root="${workdir}/root"
readonly ran_with="${root}/bin/ran-with"
readonly said='workflow web: serving http://127.0.0.1:24680/#session=STUB'
mkdir -p "${root}/scripts" "${root}/bin"
cp "${here}/e2e-server.sh" "${root}/scripts/"
printf '#!/bin/sh\necho "$*" >"%s"\necho "%s" >&2\nexit 0\n' "${ran_with}" "${said}" >"${root}/bin/workflow"
chmod +x "${root}/bin/workflow"

# port is the port each case asks the script to serve on: not the one the
# Playwright run passes, so a script that ignored it would be caught.
readonly port=24680

readonly -a git=(env -i "PATH=${PATH}" "HOME=${workdir}" GIT_CONFIG_NOSYSTEM=1 git)

# serve runs the script on a fixture path and the port, keeping its output in
# <path>.out, and prints pass or fail.
serve() {
  local path="$1"
  rm -f -- "${ran_with}"
  if "${root}/scripts/e2e-server.sh" "${path}" "${port}" >"${path}.out" 2>&1; then
    echo "pass"
  else
    echo "fail"
  fi
}

# failed records a failed case, with the script's output.
#   failed <name> <path> <what went wrong>
failed() {
  fail_case "$1" "$3"
  sed 's/^/  /' "$2.out" >&2
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
  count_case
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

# log_problem prints what the fixture's server.log lacks, or nothing when it
# holds what the server said, waiting a moment for it: tee writes it beside
# the server, which may have exited first.
log_problem() {
  local log="$1/server.log"
  for _ in {1..50}; do
    if grep -qxF -- "${said}" "${log}" 2>/dev/null; then
      return
    fi
    sleep 0.1
  done
  echo "server.log does not hold what the server said"
}

# expect_fixture wants the script to build the whole fixture at a path and
# serve it on the port, keeping what the server says in its server.log.
#   expect_fixture <name> <path>
expect_fixture() {
  local name="$1" path="$2" got problem served logged
  count_case
  got="$(serve "${path}")"

  if [[ "${got}" != "pass" ]]; then
    failed "${name}" "${path}" "want a fixture, got a refusal"
    return
  fi

  problem="$(fixture_problem "${path}")"
  served="$(cat -- "${ran_with}" 2>/dev/null || true)"
  logged="$(log_problem "${path}")"
  if [[ -n "${problem}" ]]; then
    failed "${name}" "${path}" "${problem}"
  elif [[ "${served}" != "--web --port ${port}" ]]; then
    failed "${name}" "${path}" "the server ran with '${served}', want '--web --port ${port}'"
  elif [[ -n "${logged}" ]]; then
    failed "${name}" "${path}" "${logged}"
  fi
}

# expect_raced wants the script to refuse a path someone else makes again in
# the moment between its clearing the path and its making it: an rm first on
# PATH clears the path and makes it anew, open to everyone and, when the test
# runs as root, owned by nobody. The script must stop there, serving nothing
# and making nothing inside that directory.
#   expect_raced <name> <path>
expect_raced() {
  local name="$1" path="$2" shim="${workdir}/shim" got made
  count_case
  mkdir -p "${shim}"
  {
    printf '#!/bin/sh\n'
    printf '"%s" "$@"\n' "$(command -v rm)"
    printf 'mkdir -m 777 -- "%s"\n' "${path}"
    if ((EUID == 0)); then
      printf 'chown 65534 -- "%s"\n' "${path}"
    fi
  } >"${shim}/rm"
  chmod +x "${shim}/rm"
  got="$(PATH="${shim}:${PATH}" serve "${path}")"
  made="$(find "${path}" -mindepth 1 -print -quit 2>/dev/null || true)"

  if [[ "${got}" != "fail" ]]; then
    failed "${name}" "${path}" "want a refusal, got a fixture"
  elif [[ -e "${ran_with}" ]]; then
    failed "${name}" "${path}" "the server ran"
  elif [[ -n "${made}" ]]; then
    failed "${name}" "${path}" "the script made ${made} inside a directory it did not make"
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
count_case
if [[ -e "${rebuilt}/stale.txt" ]]; then
  fail_case "a rebuild" "stale.txt survived it"
fi

expect_raced "a path made again by someone else between clearing and making" "${workdir}/raced"

finish_tests
