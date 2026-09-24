#!/usr/bin/env bash
#
# Tests for check-file-length.sh: the gate must measure real files, and must
# never report success when it measured nothing.
#
# Usage:
#   scripts/check-file-length_test.sh
set -euo pipefail

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories this test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-file-length.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  local want="$1" name="$2" dir="$3" got
  cases=$((cases + 1))

  if (cd "${dir}" && "${check}") >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
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

# --list reports what the gate measures, so a TypeScript file appears in it.
listed="${workdir}/listed"
mkdir -p "${listed}"
git -C "${listed}" init -q
soft_length >"${listed}/Panel.tsx"
git -C "${listed}" add Panel.tsx
cases=$((cases + 1))
if ! (cd "${listed}" && "${check}" --list) 2>/dev/null | grep -q 'Panel\.tsx'; then
  echo "FAIL --list names a TypeScript file: Panel.tsx is missing" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  echo "check-file-length_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-file-length_test: ${cases} case(s) passed."
