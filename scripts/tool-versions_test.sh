#!/usr/bin/env bash
#
# Tests for tool-versions.sh: it emits the --build-arg flags from a complete
# mise.toml, and fails rather than emitting a blank when the file is missing or a
# pin it needs is absent — a blank version would build the container on whatever
# "latest" happened to be.
#
# Usage:
#   scripts/tool-versions_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/tool-versions.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# expect runs the script against a mise.toml and checks its exit.
#   expect <pass|fail> <name> <mise.toml path>
expect() {
  local want="$1" name="$2" toml="$3" got
  cases=$((cases + 1))

  if MISE_TOML="${toml}" "${script}" >/dev/null 2>&1; then
    got="pass"
  else
    got="fail"
  fi

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    failures=$((failures + 1))
  fi
}

# complete writes a mise.toml holding every pin the script reads.
complete() {
  cat <<'TOML'
go = "1.27.1"
task = "3.45.4"
golangci-lint = "2.6.1"
lefthook = "2.1.14"
shellcheck = "0.11.0"
node = "24.21.0"
jq = "1.8.2"
shfmt = "3.12.0"
yamllint = "1.37.1"
typos = "1.37.2"
actionlint = "1.7.7"
hadolint = "2.14.0"
"go:golang.org/x/vuln/cmd/govulncheck" = "1.1.4"
"go:github.com/zricethezav/gitleaks/v8" = "8.29.0"
"go:github.com/rillig/gobco" = "1.7.0"
taplo = "0.10.0"
zizmor = "1.14.2"
"npm:markdownlint-cli2" = "0.18.1"
TOML
}

# A complete mise.toml yields the flags.
full="${workdir}/full.toml"
complete >"${full}"
expect pass "a complete mise.toml" "${full}"

# Its output names the build args, so the container gets the pinned versions.
flags="$(MISE_TOML="${full}" "${script}")"
cases=$((cases + 1))
if [[ "${flags}" != *"--build-arg GO_VERSION=1.27.1"* ]]; then
  echo "FAIL output names GO_VERSION: got ${flags}" >&2
  failures=$((failures + 1))
fi

# A mise.toml missing a pin the script needs fails rather than emitting a blank.
missing_pin="${workdir}/missing-pin.toml"
complete | grep -v '^go = ' >"${missing_pin}"
expect fail "a mise.toml missing a pin" "${missing_pin}"

# A mise.toml that is not there fails.
expect fail "a missing mise.toml" "${workdir}/does-not-exist.toml"

if ((failures > 0)); then
  echo "tool-versions_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "tool-versions_test: ${cases} case(s) passed."
