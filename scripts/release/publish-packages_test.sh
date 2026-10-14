#!/usr/bin/env bash
#
# Tests for publish-packages.sh. Each case runs the script on a packages
# directory with a stand-in for gh, which answers the reads a case sets up and
# writes down every call, and checks whether it committed the formula to the
# tap, and with what.
#
# Usage:
#   scripts/release/publish-packages_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/publish-packages.sh"

# shellcheck source=../lib/testing.sh
source "${here}/../lib/testing.sh"

# A gh that writes each call down, one line of its arguments, and answers as
# gh api does:
#   the latest release with GH_STUB_LATEST;
#   the formula's blob in the tap with GH_STUB_TAP_SHA, where "missing" is
#   GitHub's not found and "refused" its bad credentials;
#   a write with the commit it made, or a refusal when GH_STUB_PUT_FAILS is set.
mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${GH_STUB_LOG}"
answer() {
  case "$1" in
    missing)
      echo "gh: Not Found (HTTP 404)" >&2
      exit 1
      ;;
    refused)
      echo "gh: Bad credentials (HTTP 401)" >&2
      exit 1
      ;;
    *) printf '%s\n' "$1" ;;
  esac
}
case "$*" in
  "api --method PUT "*)
    if [[ -n "${GH_STUB_PUT_FAILS:-}" ]]; then
      echo "gh: Resource not accessible by personal access token (HTTP 403)" >&2
      exit 1
    fi
    echo '{"commit":{"sha":"0000000000000000000000000000000000000000"}}'
    ;;
  "api repos/o/workflow/releases/latest --jq .tag_name") answer "${GH_STUB_LATEST}" ;;
  "api repos/o/homebrew-tap/contents/Formula/workflow.rb --jq .sha") answer "${GH_STUB_TAP_SHA}" ;;
  *)
    echo "gh stub: nothing answers '$*'" >&2
    exit 1
    ;;
esac
STUB
chmod +x "${workdir}/bin/gh"

readonly packages="${workdir}/packages"
readonly formula="${packages}/Formula/workflow.rb"
mkdir -p "${packages}/Formula"
printf 'class Workflow < Formula\nend\n' >"${formula}"

readonly old_formula_blob="1111111111111111111111111111111111111111"

# The stand-in gh's calls in the last case that ran the script.
readonly log="${workdir}/gh.log"

# publish runs the script for a tag from a packages directory, the stand-in gh
# answering with the latest release v1.4.0 and no formula there yet, unless
# the case's VAR=value says otherwise.
#   publish <tag> <packages dir> [VAR=value...]
publish() {
  local tag="$1" dir="$2"
  shift 2
  : >"${log}"
  env PATH="${workdir}/bin:${PATH}" GH_STUB_LOG="${log}" GH_TOKEN=token \
    GH_STUB_LATEST=v1.4.0 GH_STUB_TAP_SHA=missing "$@" \
    "${script}" "${tag}" o/workflow "${dir}" o/homebrew-tap
}

# calls prints the last case's gh calls that match a text.
#   calls <text>
calls() {
  grep -F -- "$1" "${log}" || true
}

# expect_writes wants the last case to have made exactly a number of writes.
#   expect_writes <name> <count>
expect_writes() {
  local got
  got="$(grep -cF -- "api --method PUT" "${log}" || true)"
  count_case
  if ((got != $2)); then
    fail_case "$1" "want $2 write(s), got ${got}:"
    sed 's/^/    /' "${log}" >&2
  fi
}

# expect_write wants the last case to have written a file to a repository's
# path with a commit message, the file's content and, for a replacement, the
# blob it replaces.
#   expect_write <name> <repository/contents/path> <message> <file> [old blob]
expect_write() {
  local name="$1" target="$2" message="$3" content sha_arg="-f sha=" line
  content="$(base64 <"$4" | tr -d '\n')"
  line="$(calls "api --method PUT repos/${target} -f message=${message} -f content=${content}")"
  count_case
  if [[ -z "${line}" ]]; then
    fail_case "${name}" "no write of $4 to ${target} saying '${message}'"
  elif [[ -n "${5:-}" && "${line}" != *" ${sha_arg}$5" ]]; then
    fail_case "${name}" "the write to ${target} does not name the blob $5 it replaces: ${line}"
  elif [[ -z "${5:-}" && "${line}" == *"${sha_arg}"* ]]; then
    fail_case "${name}" "the write to ${target} names a blob, with none there: ${line}"
  fi
}

expect_output pass "no token publishes nothing and says so" "::warning::No PACKAGES_TOKEN" \
  publish v1.4.0 "${packages}" GH_TOKEN=
count_case
if [[ -s "${log}" ]]; then
  fail_case "no token publishes nothing and says so" "gh was run: $(<"${log}")"
fi

expect_output pass "a release that is not the latest is left alone" "v1.4.0 is not the latest release (v1.5.0 is)" \
  publish v1.4.0 "${packages}" GH_STUB_LATEST=v1.5.0
expect_writes "a release that is not the latest is left alone" 0

expect_output pass "a prerelease is left alone" "v1.5.0-rc.1 is not the latest release (v1.4.0 is)" \
  publish v1.5.0-rc.1 "${packages}"
expect_writes "a prerelease is left alone" 0

expect_output 1 "a latest release that cannot be read fails" "HTTP 401" \
  publish v1.4.0 "${packages}" GH_STUB_LATEST=refused
expect_writes "a latest release that cannot be read fails" 0

expect_output 1 "a source with no release fails" "HTTP 404" \
  publish v1.4.0 "${packages}" GH_STUB_LATEST=missing
expect_writes "a source with no release fails" 0

expect_output pass "the first release creates the formula" "o/homebrew-tap: Formula/workflow.rb now names v1.4.0" \
  publish v1.4.0 "${packages}"
expect_writes "the first release writes one file" 1
expect_write "the first release creates the formula" \
  o/homebrew-tap/contents/Formula/workflow.rb "workflow 1.4.0" "${formula}"

expect_exit pass "a later release replaces the formula" \
  publish v1.4.0 "${packages}" GH_STUB_TAP_SHA="${old_formula_blob}"
expect_write "a later release replaces the formula, naming its blob" \
  o/homebrew-tap/contents/Formula/workflow.rb "workflow 1.4.0" "${formula}" "${old_formula_blob}"

expect_output pass "a formula already as written is not committed again" \
  "o/homebrew-tap: Formula/workflow.rb already names v1.4.0" \
  publish v1.4.0 "${packages}" GH_STUB_TAP_SHA="$(git hash-object --no-filters "${formula}")"
expect_writes "a formula already as written is not committed again" 0

expect_output 1 "a read refused other than not found fails" "HTTP 401" \
  publish v1.4.0 "${packages}" GH_STUB_TAP_SHA=refused
expect_writes "a read refused other than not found fails" 0

expect_output 1 "a refused write fails" "HTTP 403" publish v1.4.0 "${packages}" GH_STUB_PUT_FAILS=1

readonly empty="${workdir}/empty"
mkdir -p "${empty}"
expect_output 1 "a formula not written fails before anything is asked" "Formula/workflow.rb was not written" \
  publish v1.4.0 "${empty}"
count_case
if [[ -s "${log}" ]]; then
  fail_case "a formula not written fails before anything is asked" "gh was run"
fi

readonly canary="canary-value-7f3a2c91e04b"
expect_exit pass "the token never reaches the output" publish v1.4.0 "${packages}" GH_TOKEN="${canary}"
count_case
if grep -qF "${canary}" "${case_output}" "${log}"; then
  fail_case "the token never reaches the output" "it was printed or passed as an argument"
fi

expect_output 2 "no arguments is a usage error" "Usage:" "${script}"
expect_output 2 "too few arguments is a usage error" "Usage:" "${script}" v1.4.0 o/workflow "${packages}"
expect_output 2 "too many arguments is a usage error" "Usage:" \
  "${script}" v1.4.0 o/workflow "${packages}" o/homebrew-tap extra

finish_tests
