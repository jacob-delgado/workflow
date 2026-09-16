#!/usr/bin/env bash
# Emit the build-container's --build-arg flags from mise.toml, so the container
# and the host install the same versions. mise.toml is the single source of
# truth; a second copy of a version string is a drift waiting to happen.
#
# Usage:  podman build $(scripts/tool-versions.sh) -f build/Dockerfile .
set -euo pipefail

readonly mise_toml="${MISE_TOML:-mise.toml}"

if [[ ! -f "${mise_toml}" ]]; then
  echo "mise.toml not found at ${mise_toml}" >&2
  exit 1
fi

# Read the value of a [tools] key: `key = "value"` or `"key" = "value"`.
pin() {
  local key="$1" escaped value
  escaped="${key//\//\\/}"
  value="$(sed -n -E "s/^\"?${escaped}\"?[[:space:]]*=[[:space:]]*\"([^\"]+)\".*/\1/p" "${mise_toml}" | head -1)"
  if [[ -z "${value}" ]]; then
    echo "no pin for '${key}' in ${mise_toml}" >&2
    return 1
  fi
  printf '%s' "${value}"
}

go_version="$(pin go)"
task_version="$(pin task)"
golangci_lint_version="$(pin golangci-lint)"
shfmt_version="$(pin shfmt)"
yamllint_version="$(pin yamllint)"
typos_version="$(pin typos)"
actionlint_version="$(pin actionlint)"
hadolint_version="$(pin hadolint)"
govulncheck_version="$(pin 'go:golang.org/x/vuln/cmd/govulncheck')"
gitleaks_version="$(pin 'go:github.com/zricethezav/gitleaks/v8')"
gobco_version="$(pin 'go:github.com/rillig/gobco')"

printf -- '--build-arg GO_VERSION=%s ' "${go_version}"
printf -- '--build-arg TASK_VERSION=%s ' "${task_version}"
printf -- '--build-arg GOLANGCI_LINT_VERSION=%s ' "${golangci_lint_version}"
printf -- '--build-arg SHFMT_VERSION=%s ' "${shfmt_version}"
printf -- '--build-arg YAMLLINT_VERSION=%s ' "${yamllint_version}"
printf -- '--build-arg TYPOS_VERSION=%s ' "${typos_version}"
printf -- '--build-arg ACTIONLINT_VERSION=%s ' "${actionlint_version}"
printf -- '--build-arg HADOLINT_VERSION=%s ' "${hadolint_version}"
printf -- '--build-arg GOVULNCHECK_VERSION=%s ' "${govulncheck_version}"
printf -- '--build-arg GITLEAKS_VERSION=%s ' "${gitleaks_version}"
printf -- '--build-arg GOBCO_VERSION=%s\n' "${gobco_version}"
