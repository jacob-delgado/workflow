#!/usr/bin/env bash
#
# Tests for which release .github/workflows/packages.yml points the Homebrew
# tap at: whichever is latest when the run starts, never one a dispatch names.
# GitHub keeps one run of the packages concurrency group pending and cancels
# the one it held, so a run for a release named by hand could take the place
# of the latest release's, and the tap would never move to it. The workflow
# takes no input, then, and its first step finds the latest release; every
# later step reads the tag from that step. The step's script is read out of
# packages.yml as the job runs it and run under bash with a stand-in for gh
# that answers the latest release's tag.
#
# Usage:
#   scripts/release/packages-workflow_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly workflow="${here}/../../.github/workflows/packages.yml"

# shellcheck source=../lib/testing.sh
source "${here}/../lib/testing.sh"

# The step's script, every line under its `run: |` indented past the key,
# with that indentation taken off.
readonly script="${workdir}/latest.sh"
awk '
  /^ *- name: Find the latest release$/ { step = 1; next }
  step && !indent && /^ *run: \|$/ { match($0, /^ */); indent = RLENGTH; next }
  indent && /[^ ]/ { match($0, /^ */); if (RLENGTH <= indent) exit }
  indent { print substr($0, indent + 3) }
' "${workflow}" >"${script}"

# A gh that answers the latest release's tag with GH_STUB_LATEST, or as
# GitHub does when a repository has no release, with "missing".
mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gh" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  "release view --repo o/workflow --json tagName --jq .tagName")
    if [[ "${GH_STUB_LATEST}" == missing ]]; then
      echo "release not found" >&2
      exit 1
    fi
    printf '%s\n' "${GH_STUB_LATEST}"
    ;;
  *)
    echo "gh stub: nothing answers '$*'" >&2
    exit 1
    ;;
esac
STUB
chmod +x "${workdir}/bin/gh"

count_case
if ! grep -q 'gh release view' "${script}"; then
  fail_case "the step that finds the latest release" "it was not found in packages.yml: $(cat "${script}")"
fi

# find_latest runs the step as Actions runs a bash step, with the latest
# release given, writing its outputs to a file of its own.
#   find_latest <latest tag>
find_latest() {
  : >"${workdir}/output"
  env PATH="${workdir}/bin:${PATH}" GH_STUB_LATEST="$1" GITHUB_REPOSITORY=o/workflow \
    GITHUB_OUTPUT="${workdir}/output" bash --noprofile --norc -eo pipefail "${script}"
}

expect_exit pass "the latest release" find_latest v1.6.0
count_case
if [[ "$(cat "${workdir}/output")" != "tag=v1.6.0" ]]; then
  fail_case "the latest release" "the step's output is not its tag: $(cat "${workdir}/output")"
fi

expect_exit fail "no release to read" find_latest missing
count_case
if grep -q '^tag=' "${workdir}/output"; then
  fail_case "no release to read" "the step named a tag: $(cat "${workdir}/output")"
fi

count_case
if grep -nE '^ *inputs:|inputs\.' "${workflow}"; then
  fail_case "a release named by hand" "packages.yml takes an input, which a run could publish in place of the latest"
fi

finish_tests
