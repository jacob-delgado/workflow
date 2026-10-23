#!/usr/bin/env bash
#
# Tests for check-playwright-image.sh: it passes when every Playwright image
# the workflows run in names the version web/yarn.lock resolves
# @playwright/test to and is pinned by digest, and fails on a drifted version,
# an image named by its tag alone, a lock it cannot read, or a tree with no
# workflow to read.
#
# Usage:
#   scripts/check-playwright-image_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-playwright-image.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly digest="sha256:06a9939e57531807f8d5fd76ce44b53165ffb7d7501d87ab10e285c20b1e971f"

# repo makes a git repository whose web/yarn.lock resolves @playwright/test to
# the version given.
#   repo <dir> <version>
repo() {
  local dir="$1" version="$2"
  mkdir -p "${dir}/web" "${dir}/.github/workflows"
  git -C "${dir}" init --quiet
  cat >"${dir}/web/yarn.lock" <<EOF
"@playwright/test@npm:^${version}":
  version: ${version}
  resolution: "@playwright/test@npm:${version}"
EOF
  git -C "${dir}" add web/yarn.lock
}

# workflow writes a workflow whose job runs in the image given, and stages it.
#   workflow <dir> <file> <image>
workflow() {
  local dir="$1" file="$2" image="$3"
  cat >"${dir}/.github/workflows/${file}" <<EOF
jobs:
  e2e:
    runs-on: ubuntu-latest
    container:
      image: ${image}
EOF
  git -C "${dir}" add ".github/workflows/${file}"
}

# expect_says runs the gate in dir and holds it to an exit and to words it says.
#   expect_says <pass|fail|status> <name> <words> <dir>
expect_says() {
  expect_output "$1" "$2" "$3" run_in "$4" "${check}"
}

# The image names the lock's version and its digest: the gate passes.
agree="${workdir}/agree"
repo "${agree}" 1.64.0
workflow "${agree}" ci.yml "mcr.microsoft.com/playwright:v1.64.0-noble@${digest}"
expect_says pass "an image in step with the lock" "1.64.0" "${agree}"

# A bump of @playwright/test that leaves the image behind.
drift="${workdir}/drift"
repo "${drift}" 1.65.0
workflow "${drift}" ci.yml "mcr.microsoft.com/playwright:v1.64.0-noble@${digest}"
expect_says fail "an image behind the lock" "v1.64.0" "${drift}"

# A tag alone can be moved to other browsers under the same name.
unpinned="${workdir}/unpinned"
repo "${unpinned}" 1.64.0
workflow "${unpinned}" ci.yml "mcr.microsoft.com/playwright:v1.64.0-noble"
expect_says fail "an image named by its tag alone" "digest" "${unpinned}"

# Every image counts: one in step does not cover another that drifted.
second="${workdir}/second"
repo "${second}" 1.64.0
workflow "${second}" ci.yml "mcr.microsoft.com/playwright:v1.64.0-noble@${digest}"
workflow "${second}" other.yml "mcr.microsoft.com/playwright:v1.63.0-noble@${digest}"
expect_says fail "a second workflow's image drifted" "other.yml" "${second}"

# A lock that resolves no @playwright/test leaves nothing to compare to.
nolock="${workdir}/nolock"
repo "${nolock}" 1.64.0
printf 'other@npm:1.0.0:\n  version: 1.0.0\n' >"${nolock}/web/yarn.lock"
workflow "${nolock}" ci.yml "mcr.microsoft.com/playwright:v1.64.0-noble@${digest}"
expect_says 2 "a lock with no @playwright/test" "yarn.lock" "${nolock}"

# No tracked workflow: the gate read nothing, so it must not pass.
empty="${workdir}/empty"
repo "${empty}" 1.64.0
expect_says 2 "no workflow to read" "no workflow" "${empty}"

finish_tests
