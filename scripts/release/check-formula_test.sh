#!/usr/bin/env bash
#
# Tests for check-formula.sh. Each case runs the script on a formula with a
# stand-in for brew, which writes down every call, keeps its taps under the
# test's own directory and passes or fails the audit as the case sets up, and
# checks what the script asked of brew, what it said, and what it left behind.
#
# Usage:
#   scripts/release/check-formula_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script="${here}/check-formula.sh"

# shellcheck source=../lib/testing.sh
source "${here}/../lib/testing.sh"

# A developer's shell often exports HOMEBREW_ variables of its own, which would
# let a case pass on one the script no longer sets: brew sees only the script's.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(compgen -e HOMEBREW_)

# A brew that writes each call down, one line of its arguments, with the
# HOMEBREW_ variables it was run with beside it, and answers as brew does:
#   --repository <user>/<repo> with that tap's directory under BREW_STUB_TAPS,
#   whether or not it is there, or with BREW_STUB_REPOSITORY when that is set,
#   or by failing when BREW_STUB_REPOSITORY_FAILS is set;
#   tap-new --no-git <user>/<repo> by making that directory and its Formula
#   directory, refusing a tap already there, or failing when
#   BREW_STUB_TAP_NEW_FAILS is set;
#   --version with two lines, as brew does, or by failing when
#   BREW_STUB_VERSION_FAILS is set;
#   audit by keeping the formula its last argument names, <user>/<repo>/<name>,
#   in BREW_STUB_AUDITED, and failing, saying BREW_STUB_AUDIT_SAYS, when that
#   is set.
# Anything else is refused, so a call the script should not make fails it.
mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/brew" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${BREW_STUB_LOG}"
env | grep '^HOMEBREW_' | sort | tr '\n' ' ' >>"${BREW_STUB_ENV}"
echo >>"${BREW_STUB_ENV}"
tap_dir() {
  printf '%s/%s/homebrew-%s\n' "${BREW_STUB_TAPS}" "${1%%/*}" "${1#*/}"
}
case "$1" in
  --repository)
    if [[ -n "${BREW_STUB_REPOSITORY_FAILS:-}" ]]; then
      echo "Error: No available tap $2." >&2
      exit 1
    fi
    printf '%s\n' "${BREW_STUB_REPOSITORY-$(tap_dir "$2")}"
    ;;
  --version)
    if [[ -n "${BREW_STUB_VERSION_FAILS:-}" ]]; then
      echo "Error: Homebrew is broken" >&2
      exit 1
    fi
    printf 'Homebrew 9.9.9\nHomebrew/homebrew-core (git revision 0abc; last commit 2026-10-14)\n'
    ;;
  tap-new)
    dir="$(tap_dir "$3")"
    if [[ -n "${BREW_STUB_TAP_NEW_FAILS:-}" ]]; then
      echo "Error: Invalid tap name '$3'" >&2
      exit 1
    fi
    if [[ "$2" != "--no-git" || -e "${dir}" ]]; then
      echo "Error: Tap is already installed!" >&2
      exit 1
    fi
    mkdir -p "${dir}/Formula"
    echo "==> Created $3"
    ;;
  audit)
    full="${*: -1}"
    cp "$(tap_dir "${full%/*}")/Formula/${full##*/}.rb" "${BREW_STUB_AUDITED}"
    if [[ -n "${BREW_STUB_AUDIT_SAYS:-}" ]]; then
      printf '%s\n' "${BREW_STUB_AUDIT_SAYS}"
      exit 1
    fi
    ;;
  *)
    echo "brew stub: nothing answers '$*'" >&2
    exit 1
    ;;
esac
STUB
chmod +x "${workdir}/bin/brew"

readonly taps="${workdir}/taps"
readonly log="${workdir}/brew.log"
readonly brew_env="${workdir}/brew.env"
readonly audited="${workdir}/audited.rb"

# A tap of workflow-check's that is not the script's, there before every case:
# the script removes the tap it made and nothing beside it.
readonly other="${taps}/workflow-check/homebrew-other"

readonly formula="${workdir}/workflow.rb"
printf 'class Workflow < Formula\nend\n' >"${formula}"

readonly said="workflow:
  * Stable: version 0.0.0 is redundant with version scanned from URL
Error: 1 problem in 1 formula detected."

# check runs the script on a formula with the stand-in brew, which starts
# with no tap but the other one and passes the audit, unless the case's
# VAR=value says otherwise.
#   check <formula> [VAR=value...]
check() {
  local file="$1"
  shift
  : >"${log}"
  : >"${brew_env}"
  rm -rf "${taps}" "${audited}"
  mkdir -p "${other}/Formula"
  printf 'kept\n' >"${other}/Formula/other.rb"
  env PATH="${workdir}/bin:${PATH}" BREW_STUB_LOG="${log}" BREW_STUB_ENV="${brew_env}" \
    BREW_STUB_TAPS="${taps}" BREW_STUB_AUDITED="${audited}" "$@" "${script}" "${file}"
}

# calls prints the last case's brew calls whose first argument is a command.
#   calls <command>
calls() {
  awk -v command="$1" '$1 == command' "${log}"
}

# made_tap prints the tap the last case asked brew to make.
made_tap() {
  calls tap-new | awk '{ print $NF }'
}

# expect_said wants a text in what the last case printed, as a case of its own.
#   expect_said <name> <text>
expect_said() {
  count_case
  if [[ "$(<"${case_output}")" != *"$2"* ]]; then
    fail_case "$1" "it did not say '$2':"
    show_output
  fi
}

# expect_only_other_tap_left wants the other tap, as it was, to be the only
# tap's directory left in the stand-in brew's taps, as a case of its own.
#   expect_only_other_tap_left <name>
expect_only_other_tap_left() {
  local left kept
  left="$(find "${taps}" -mindepth 2 -maxdepth 2)"
  kept="$(cat "${other}/Formula/other.rb" 2>/dev/null)" || true
  count_case
  if [[ "${left}" != "${other}" || "${kept}" != "kept" ]]; then
    fail_case "$1" "it left '${left//"${workdir}/"/}', the other tap's formula holding '${kept}'"
  fi
}

# expect_no_calls wants the last case to have run brew for nothing but a
# command in a list, as a case of its own.
#   expect_no_calls <name> [command...]
expect_no_calls() {
  local name="$1" others
  shift
  others="$(awk -v allowed=" $* " 'index(allowed, " " $1 " ") == 0' "${log}")"
  count_case
  if [[ -n "${others}" ]]; then
    fail_case "${name}" "brew was run for $(tr '\n' ';' <<<"${others}")"
  fi
}

expect_output pass "a formula brew's audit passes is passed" "passes brew audit --strict" check "${formula}"
count_case
if ! cmp -s "${formula}" "${audited}"; then
  fail_case "the formula audited is the one given" "brew audited something else, or nothing"
fi
expect_only_other_tap_left "the tap made for a formula that passes is removed, and nothing beside it"

expect_output 1 "a formula the audit fails fails, saying what brew said" "${said}" \
  check "${formula}" BREW_STUB_AUDIT_SAYS="${said}"
expect_said "a formula the audit fails names the likely cause and the script to update" \
  "Homebrew most likely changed a rule, and scripts/release/package-manifests.sh needs updating to keep it"
expect_only_other_tap_left "the tap made for a formula that fails is removed, and nothing beside it"

expect_exit pass "the audit is the only check run" check "${formula}"
tap="$(made_tap)"
made="$(calls tap-new)"
audit="$(calls audit)"
count_case
if [[ ! "${tap}" =~ ^workflow-check/f[0-9]+-[0-9]+$ ]]; then
  fail_case "the tap is made as one of workflow-check's" "it made '${tap}'"
fi
count_case
if [[ "${made}" != "tap-new --no-git ${tap}" ]]; then
  fail_case "the tap is made with no git repository" "${made}"
fi
count_case
if [[ "${audit}" != "audit --strict --formula ${tap}/workflow" ]]; then
  fail_case "the audit is strict and names the formula in full" "${audit}"
fi
expect_no_calls "the audit is the only check run" --repository tap-new --version audit

for variable in HOMEBREW_NO_AUTO_UPDATE HOMEBREW_NO_ANALYTICS HOMEBREW_NO_ENV_HINTS HOMEBREW_DEVELOPER; do
  count_case
  if [[ ! -s "${brew_env}" ]] || grep -q -v -F -e "${variable}=1 " "${brew_env}"; then
    fail_case "brew runs with ${variable} set" "$(<"${brew_env}")"
  fi
done
count_case
if [[ ! -s "${brew_env}" ]] || grep -q -F -e "HOMEBREW_NO_INSTALL_FROM_API=" "${brew_env}"; then
  fail_case "brew runs with HOMEBREW_NO_INSTALL_FROM_API unset" "$(<"${brew_env}")"
fi

expect_output pass "the Homebrew that audits is named" "with Homebrew 9.9.9" check "${formula}"
count_case
if [[ "$(<"${case_output}")" == *"git revision"* ]]; then
  fail_case "the Homebrew that audits is named in one line" "it said all of brew --version:"
  show_output
fi

expect_output 1 "a brew that cannot say its version fails, saying so" "could not say its version" \
  check "${formula}" BREW_STUB_VERSION_FAILS=1
expect_no_calls "a brew that cannot say its version audits nothing" --repository tap-new --version
expect_only_other_tap_left "the tap made for a brew that cannot say its version is removed"

expect_exit pass "two runs make two taps" check "${formula}"
first="$(made_tap)"
expect_exit pass "two runs make two taps" check "${formula}"
second="$(made_tap)"
count_case
if [[ -z "${first}" || "${first}" == "${second}" ]]; then
  fail_case "two runs make two taps" "both made '${first}'"
fi

readonly existing="${workdir}/existing"
mkdir -p "${existing}/Formula"
printf 'kept\n' >"${existing}/Formula/workflow.rb"
expect_output 1 "a tap already there is refused" "already exists" \
  check "${formula}" BREW_STUB_REPOSITORY="${existing}"
count_case
if [[ "$(<"${existing}/Formula/workflow.rb")" != "kept" ]]; then
  fail_case "a tap already there is left as it was" "its formula now holds $(<"${existing}/Formula/workflow.rb")"
fi
expect_no_calls "a tap already there is neither made nor audited" --repository

expect_output 1 "a tap brew cannot place fails, saying so" "brew could not say where" \
  check "${formula}" BREW_STUB_REPOSITORY_FAILS=1
expect_said "a tap brew cannot place fails, saying what brew said" "No available tap"
expect_no_calls "a tap brew cannot place is neither made nor audited" --repository

for placed in "" "homebrew-relative"; do
  expect_output 1 "a tap brew places at '${placed}' is refused" "not an absolute path" \
    check "${formula}" BREW_STUB_REPOSITORY="${placed}"
  expect_no_calls "a tap brew places at '${placed}' is neither made nor audited" --repository
done

expect_output 1 "a tap brew cannot make fails, auditing nothing" "Invalid tap name" \
  check "${formula}" BREW_STUB_TAP_NEW_FAILS=1
expect_no_calls "a tap brew cannot make fails, auditing nothing" --repository tap-new

readonly no_brew="${workdir}/no-brew"
mkdir -p "${no_brew}"
expect_output 1 "no brew on PATH fails, saying so" "brew is not on PATH" \
  env PATH="${no_brew}" "${BASH}" "${script}" "${formula}"

expect_output 2 "a formula that is not there is a usage error" "cannot read" check "${workdir}/absent.rb"
expect_no_calls "a formula that is not there runs no brew"
expect_output 2 "no arguments is a usage error" "Usage:" "${script}"
expect_output 2 "too many arguments is a usage error" "Usage:" "${script}" "${formula}" extra

finish_tests
