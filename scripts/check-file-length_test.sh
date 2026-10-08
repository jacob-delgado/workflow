#!/usr/bin/env bash
#
# Tests for check-file-length.sh: the gate must measure real files, and must
# never report success when it measured nothing.
#
# Usage:
#   scripts/check-file-length_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-file-length.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  expect_exit "$1" "$2" run_in "$3" "${check}"
}

# over_length writes a file of 901 lines, past the 800-line hard ceiling.
over_length() {
  awk 'BEGIN { for (i = 0; i < 901; i++) print "x" }'
}

# soft_length writes a file of 600 lines: past the 500-line soft target but
# within the 800-line hard ceiling, so it warns without failing the gate.
soft_length() {
  awk 'BEGIN { for (i = 0; i < 600; i++) print "x" }'
}

# lines writes a file of the given number of lines.
#   lines <count>
lines() {
  awk -v count="$1" 'BEGIN { for (i = 0; i < count; i++) print "x" }'
}

# repository_with makes a repository tracking a short Go file and a file of
# the given length at path, and prints where it is.
#   repository_with <name> <path> <lines>
repository_with() {
  local dir="${workdir}/$1"
  mkdir -p "$(dirname "${dir}/$2")"
  git -C "${dir}" init -q
  printf 'package x\n' >"${dir}/small.go"
  lines "$3" >"${dir}/$2"
  git -C "${dir}" add small.go "$2"
  printf '%s\n' "${dir}"
}

# A directory that is not a git repository: git cannot list its files, and the
# gate must refuse rather than pass on nothing measured.
notrepo="${workdir}/notrepo"
mkdir -p "${notrepo}"
over_length >"${notrepo}/big.go"
expect fail "a directory that is not a repository" "${notrepo}"

# A repository within the ceiling passes.
short="${workdir}/short"
mkdir -p "${short}"
git -C "${short}" init -q
printf 'package x\n' >"${short}/small.go"
git -C "${short}" add small.go
expect pass "a repository within the ceiling" "${short}"

# A repository with a file past the hard ceiling fails, which is the gate's
# whole point.
long="${workdir}/long"
mkdir -p "${long}"
git -C "${long}" init -q
over_length >"${long}/big.go"
git -C "${long}" add big.go
expect fail "a repository with a file past the hard ceiling" "${long}"

# A file past the soft target but within the hard ceiling only warns, so the
# gate still passes.
soft="${workdir}/soft"
mkdir -p "${soft}"
git -C "${soft}" init -q
soft_length >"${soft}/medium.go"
git -C "${soft}" add medium.go
expect pass "a file past the soft target but within the ceiling" "${soft}"

# An over-length GENERATED file is exempt: length is not a design signal for
# machine-written code, and a drift gate guards it instead.
generated="${workdir}/generated"
mkdir -p "${generated}"
git -C "${generated}" init -q
over_length >"${generated}/big.gen.go"
git -C "${generated}" add big.gen.go
expect pass "an over-length generated file" "${generated}"

# The web frontend is measured like the Go: an over-length TypeScript file
# fails, whether it holds JSX or not. Each repository also tracks a short Go
# file, so a gate that ignored TypeScript would pass on it rather than refuse
# for having measured nothing — the failure has to come from the long file.
for extension in tsx ts; do
  typescript="${workdir}/typescript-${extension}"
  mkdir -p "${typescript}"
  git -C "${typescript}" init -q
  printf 'package x\n' >"${typescript}/small.go"
  over_length >"${typescript}/big.${extension}"
  git -C "${typescript}" add small.go "big.${extension}"
  expect fail "an over-length .${extension} file" "${typescript}"
done

# A frontend with no Go in it still has something to measure, so the gate
# passes rather than refusing for an empty file list.
frontend="${workdir}/frontend"
mkdir -p "${frontend}"
git -C "${frontend}" init -q
printf 'export const x = 1\n' >"${frontend}/small.ts"
git -C "${frontend}" add small.ts
expect pass "a repository with only a short TypeScript file" "${frontend}"

# The hey-api client is machine-written from api/openapi.yaml and guarded by
# `yarn gen:check`, so its tree is exempt by directory: its files are not all
# named .gen.ts, and types.gen.ts alone runs past the ceiling.
sdk="${workdir}/sdk"
mkdir -p "${sdk}/web/src/api/generated"
git -C "${sdk}" init -q
printf 'package x\n' >"${sdk}/small.go"
over_length >"${sdk}/web/src/api/generated/index.ts"
git -C "${sdk}" add small.go web/src/api/generated/index.ts
expect pass "an over-length file in the generated web client" "${sdk}"

# The exemption is that one tree, not the frontend around it: a hand-written
# file beside the generated client is measured like any other.
beside="${workdir}/beside"
mkdir -p "${beside}/web/src/api"
git -C "${beside}" init -q
printf 'package x\n' >"${beside}/small.go"
over_length >"${beside}/web/src/api/client.ts"
git -C "${beside}" add small.go web/src/api/client.ts
expect fail "an over-length hand-written file beside the generated client" "${beside}"

# A test file is held to a lower ceiling than source, 700 lines, whichever
# language it is in: a test file grows a case at a time, and past that it
# holds the cases of more than one behavior.
for path in big_test.go big_test.sh Big.test.ts Big.test.tsx web/e2e/big.spec.ts web/e2e/helpers.ts; do
  expect fail "a test file past the test ceiling: ${path}" \
    "$(repository_with "test-${path//\//-}" "${path}" 701)"
done

# A test file at the test ceiling passes.
expect pass "a test file at the test ceiling" "$(repository_with at-test-ceiling at_test.go 700)"

# A source file the same length is within its own ceiling.
expect pass "a source file past the test ceiling" "$(repository_with source-past big.go 701)"

# --list marks a test file past its ceiling, and only that one, as over.
over_test="$(repository_with listed-tests big_test.go 701)"
lines 701 >"${over_test}/big.go"
git -C "${over_test}" add big.go
expect_output pass "--list marks a test file past its ceiling" "  701  big_test.go  <= OVER" \
  run_in "${over_test}" "${check}" --list
expect_output pass "--list marks a source file the same length soft" "  701  big.go  <= soft" \
  run_in "${over_test}" "${check}" --list

# --list reports what the gate measures, so a TypeScript file appears in it.
listed="${workdir}/listed"
mkdir -p "${listed}"
git -C "${listed}" init -q
soft_length >"${listed}/Panel.tsx"
git -C "${listed}" add Panel.tsx
expect_output pass "--list names a TypeScript file" "Panel.tsx" run_in "${listed}" "${check}" --list

finish_tests
