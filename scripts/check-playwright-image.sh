#!/usr/bin/env bash
#
# Fail when a Playwright image the workflows run in drifts from the Playwright
# web/yarn.lock resolves, or is named by its tag alone.
#
# Usage:
#   scripts/check-playwright-image.sh          # the gate
#   scripts/check-playwright-image.sh --list   # each image and where it is
#
# CI's end-to-end jobs run in mcr.microsoft.com/playwright, whose browsers are
# the ones its version of Playwright drives: a job whose image names another
# version than the @playwright/test the web app installs finds no browser it
# can launch. Dependabot moves @playwright/test in web/yarn.lock but not an
# image a workflow names, so this gate fails the bump until the image moves
# with it (TRADE-58). Every image must also be pinned by its digest, which a
# tag is not: the same tag can be pushed again with other contents.
#
# Paths are read from the current directory, and the workflows are the ones git
# tracks under .github/workflows, so `task` runs it from the repository root and
# its own test runs it against a fixture repository. It exits 1 on a drifted or
# unpinned image, and 2 when it cannot read the lock's version or git lists no
# workflow, rather than pass having compared nothing.
set -euo pipefail

readonly lock="web/yarn.lock"
readonly image_pattern='mcr\.microsoft\.com/playwright:[^[:space:]"'"'"']+'
readonly digest_pattern='@sha256:[0-9a-f]{64}$'

refuse() {
  echo "check-playwright-image: $*" >&2
  exit 2
}

# locked_version is the version the lock resolves @playwright/test to.
locked_version() {
  [[ -f "${lock}" ]] || refuse "no ${lock} to read the Playwright version from"
  awk '
    /^"@playwright\/test@/ { inside = 1; next }
    inside && /^  version: / { print $2; exit }
    /^[^ ]/ { inside = 0 }
  ' "${lock}"
}

# images prints each Playwright image a tracked workflow names, as file:image.
images() {
  local file
  while IFS= read -r file; do
    grep -oE "${image_pattern}" "${file}" | sed "s|^|${file}:|" || true
  done <<<"${workflows}"
}

if [[ -n "${1:-}" && "$1" != "--list" ]]; then
  sed -n '3,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

version="$(locked_version)"
[[ -n "${version}" ]] || refuse "${lock} resolves no @playwright/test"

workflows="$(git ls-files -- '.github/workflows/*.yml' '.github/workflows/*.yaml')"
[[ -n "${workflows}" ]] || refuse "git lists no workflow under .github/workflows"

found="$(images)"

if [[ "${1:-}" == "--list" ]]; then
  echo "${lock}: @playwright/test ${version}"
  echo "${found:-no Playwright image}"
  exit 0
fi

bad=()
while IFS= read -r entry; do
  [[ -n "${entry}" ]] || continue
  file="${entry%%:*}"
  image="${entry#*:}"
  tag="${image#mcr.microsoft.com/playwright:}"
  tag="${tag%%@*}"

  if [[ "${tag}" != "v${version}" && "${tag}" != "v${version}-"* ]]; then
    bad+=("${file}: ${image} is not Playwright ${version}")
  elif ! [[ "${image}" =~ ${digest_pattern} ]]; then
    bad+=("${file}: ${image} is not pinned by its digest (@sha256:...)")
  fi
done <<<"${found}"

if ((${#bad[@]} > 0)); then
  echo "check-playwright-image: ${lock} resolves @playwright/test ${version}, and:" >&2
  printf '  %s\n' "${bad[@]}" >&2
  echo "Move each image to mcr.microsoft.com/playwright:v${version}-<distro>@sha256:<its index digest>." >&2
  exit 1
fi

count="$(grep -c . <<<"${found}" || true)"
echo "check-playwright-image: ${count} image(s) run Playwright ${version}, each pinned by its digest."
