#!/usr/bin/env bash
# Audit a Homebrew formula as Homebrew audits a tap's.
#
# Usage:
#   check-formula.sh <formula.rb>
#
# Copies <formula.rb>, as package-manifests.sh writes it, into a tap made for
# the check, as Formula/workflow.rb, and runs `brew audit --strict` on it by
# its full name, which runs brew's style cops too. brew style is not run: on a
# tap it also lints the workflows tap-new writes there, installing shellcheck
# and actionlint to do it. Naming the formula in full is also what lets
# Homebrew load it from a tap nobody has trusted, without adding to the trust
# store. brew runs in developer mode, as homebrew-core's own CI runs it: an
# audit otherwise passes a formula calling something Homebrew has deprecated,
# saying nothing, until Homebrew disables it and the published formula breaks.
# The audit loads the formula without running its install or test block, so a
# call deprecated there still passes.
#
# The tap is workflow-check/f<pid>-<random>, refused if Homebrew already has a
# directory for it or cannot say where one would go as an absolute path, and
# is made with tap-new --no-git. Once made, its directory is removed on exit,
# directly: brew untap can refuse a tap that is not trusted. Nothing else is
# removed.
#
# It says which Homebrew audits before the audit runs, since CI's is whatever
# the runner image ships (TRADE-42 in TECH_DEBT.md), so a failure names the
# version that judged it.
#
# Exits 0 when the audit passes; 1 when it fails, when brew is not on PATH or
# cannot say its version, or when the tap cannot be made; and 2 for a usage
# error. Run by ci.yml's Cross-compile job on the formula it writes for every
# change.
set -euo pipefail

readonly script="scripts/release/check-formula.sh"
readonly tap="workflow-check/f$$-${RANDOM}"

export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_ENV_HINTS=1 HOMEBREW_DEVELOPER=1

# usage says how to run this, and exits 2.
#   usage [reason]
usage() {
  if (($# > 0)); then
    echo "$1" >&2
  fi
  echo "Usage:" >&2
  echo "  ${script} <formula.rb>" >&2
  exit 2
}

# fail says why the formula is not passed, and exits 1.
#   fail <reason>
fail() {
  echo "$1" >&2
  exit 1
}

# make_tap makes the tap, refusing one Homebrew already has a directory for,
# and prints its directory. It runs in a command substitution, where errexit
# is off, so each step's failure is checked here.
make_tap() {
  local dir made
  if ! dir="$(brew --repository "${tap}")"; then
    fail "brew could not say where the tap ${tap} would go; nothing is audited."
  fi
  if [[ "${dir}" != /* ]]; then
    fail "brew would put the tap ${tap} at '${dir}', not an absolute path, so it is not made or removed; nothing is audited."
  fi
  if [[ -e "${dir}" ]]; then
    fail "${dir} already exists, so ${tap} is not made or removed; nothing is audited."
  fi
  if ! made="$(brew tap-new --no-git "${tap}" 2>&1)"; then
    printf '%s\n' "${made}" >&2
    fail "brew tap-new failed for ${tap}, so nothing is audited; anything it made in ${dir} stays."
  fi
  printf '%s\n' "${dir}"
}

main() {
  if (($# != 1)); then
    usage
  fi
  local formula="$1"
  if [[ ! -f "${formula}" || ! -r "${formula}" ]]; then
    usage "cannot read the formula ${formula}"
  fi
  if ! command -v brew >/dev/null; then
    fail "brew is not on PATH, so ${formula} cannot be audited."
  fi

  tap_dir="$(make_tap)"
  readonly tap_dir
  trap 'rm -rf "${tap_dir}"' EXIT
  cp "${formula}" "${tap_dir}/Formula/workflow.rb"

  local version
  if ! version="$(brew --version)"; then
    fail "brew could not say its version, so ${formula} is not audited."
  fi
  echo "Auditing ${formula} with ${version%%$'\n'*}"
  if ! brew audit --strict --formula "${tap}/workflow"; then
    fail "If brew named a problem in the formula above, Homebrew most likely changed a rule, and scripts/release/package-manifests.sh needs updating to keep it."
  fi
  echo "${formula} passes brew audit --strict"
}

main "$@"
