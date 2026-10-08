#!/usr/bin/env bash
#
# Tests for check-dependency-age.sh: a version a change adds to go.mod,
# web/yarn.lock or mise.toml passes once it was published at least seven days
# before, and fails, by name, when it is younger, unless the exceptions file
# lists it with the advisory it fixes. A version whose publish date cannot be
# learned fails, versions the change leaves alone are not looked up, and a base
# git cannot resolve is refused with a status of its own.
#
# A stand-in curl answers each registry URL from a fixture file named for it,
# and fails as curl -f does for any other, so no case reaches the network.
# DEPENDENCY_AGE_NOW fixes "now" at 2026-10-08T00:00:00Z.
#
# Usage:
#   scripts/check-dependency-age_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-dependency-age.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly now=1791417600 # 2026-10-08T00:00:00Z
readonly old="2026-09-20T10:00:00Z" young="2026-10-06T10:00:00.123Z"

readonly stub_bin="${workdir}/bin" fixtures="${workdir}/fixtures"
mkdir -p "${stub_bin}" "${fixtures}"
cat >"${stub_bin}/curl" <<'STUB'
#!/usr/bin/env bash
url="${*: -1}"
printf '%s\n' "${url}" >>"${CURL_STUB_LOG}"
fixture="${CURL_STUB_FIXTURES}/$(printf '%s' "${url}" | tr -c 'A-Za-z0-9.' '_')"
if [[ -f "${fixture}" ]]; then
  cat "${fixture}"
else
  exit 22
fi
STUB
chmod +x "${stub_bin}/curl"

# answer records what the stand-in curl returns for a URL.
#   answer <url> <body>
answer() {
  printf '%s\n' "$2" >"${fixtures}/$(printf '%s' "$1" | tr -c 'A-Za-z0-9.' '_')"
}

answer "https://proxy.golang.org/example.com/old/@v/v1.2.0.info" "{\"Version\":\"v1.2.0\",\"Time\":\"${old}\"}"
answer "https://proxy.golang.org/example.com/young/@v/v1.3.0.info" "{\"Version\":\"v1.3.0\",\"Time\":\"${young}\"}"
answer "https://proxy.golang.org/github.com/!burnt!sushi/toml/@v/v1.6.0.info" "{\"Version\":\"v1.6.0\",\"Time\":\"${old}\"}"
answer "https://registry.npmjs.org/left-pad" "{\"time\":{\"1.4.0\":\"${old}\",\"1.5.0\":\"${young}\"}}"
answer "https://registry.npmjs.org/@scope%2Fwidget" "{\"time\":{\"2.0.0\":\"${old}\"}}"
answer "https://proxy.golang.org/golang.org/toolchain/@v/v0.0.1-go1.27.2.linux-amd64.info" "{\"Time\":\"${young}\"}"
answer "https://proxy.golang.org/github.com/rillig/gobco/@v/v1.4.0.info" "{\"Time\":\"${old}\"}"
answer "https://proxy.golang.org/golang.org/x/vuln/@v/v1.9.0.info" "{\"Time\":\"${young}\"}"
answer "https://nodejs.org/dist/index.json" '[{"version":"v24.22.0","date":"2026-09-01"}]'
answer "https://pypi.org/pypi/yamllint/1.39.0/json" "{\"urls\":[{\"upload_time_iso_8601\":\"${young}\"}]}"
answer "https://api.github.com/repos/koalaman/shellcheck/releases/tags/v0.12.0" "{\"published_at\":\"${old}\"}"
answer "https://api.github.com/repos/jqlang/jq/releases/tags/jq-1.9.0" "{\"published_at\":\"${young}\"}"

# repo makes a repository whose main branch holds a go.mod, a web/yarn.lock
# and a mise.toml, then a branch commit that applies a change to them.
#   repo <name> <change-command> [arg...]
repo() {
  local dir="${workdir}/$1"
  shift
  mkdir -p "${dir}/web" "${dir}/scripts"
  git -C "${dir}" init -q --initial-branch=main
  printf 'module example\n\ngo 1.27\n\nrequire (\n\texample.com/kept v1.0.0\n)\n' >"${dir}/go.mod"
  printf '"kept@npm:^1.0.0":\n  version: 1.0.0\n  resolution: "kept@npm:1.0.0"\n' >"${dir}/web/yarn.lock"
  printf 'min_version = "2026.9.3"\n\n[tools]\ngo = "1.27.1"\n' >"${dir}/mise.toml"
  printf '# advisories\n' >"${dir}/scripts/dependency-age-exceptions.txt"
  git -C "${dir}" add -A
  git -C "${dir}" -c user.email=t@example.com -c user.name=t commit -q -m base
  git -C "${dir}" switch -q -c change
  (cd "${dir}" && "$@")
  git -C "${dir}" add -A
  git -C "${dir}" -c user.email=t@example.com -c user.name=t commit -q -m change
  printf '%s' "${dir}"
}

# require_go adds a requirement to go.mod.
require_go() {
  sed -i.bak "s|^)|\t$1 $2\n)|" go.mod && rm go.mod.bak
}

# resolve_npm adds a resolution to web/yarn.lock.
resolve_npm() {
  printf '\n"%s@npm:^%s":\n  version: %s\n  resolution: "%s@npm:%s"\n' "$1" "$2" "$2" "$1" "$2" >>web/yarn.lock
}

# pin_tool adds or changes a [tools] pin in mise.toml.
pin_tool() {
  printf '"%s" = "%s"\n' "$1" "$2" >>mise.toml
}

# except lists a version in the exceptions file.
except() {
  printf '%s\n' "$*" >>scripts/dependency-age-exceptions.txt
}

# young_go_with_advisory requires the young module and lists it as a fix.
young_go_with_advisory() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0 GHSA-abcd-1234-wxyz
}

# young_go_without_advisory lists the young module with no advisory.
young_go_without_advisory() {
  require_go example.com/young v1.3.0
  except example.com/young v1.3.0
}

# mise_tools pins mise tools of several kinds, all a week old.
mise_tools() {
  pin_tool node 24.22.0
  pin_tool go:github.com/rillig/gobco 1.4.0
  pin_tool shellcheck 0.12.0
}

# age runs the check in a repository against main, with the stand-in curl.
#   age <dir> [base]
age() {
  : >"$1.curl"
  (cd "$1" && env PATH="${stub_bin}:${PATH}" CURL_STUB_LOG="$1.curl" CURL_STUB_FIXTURES="${fixtures}" \
    DEPENDENCY_AGE_NOW="${now}" "${check}" "${2:-main}")
}

go_old="$(repo go-old require_go example.com/old v1.2.0)"
expect_output pass "a Go module a week old" "example.com/old v1.2.0" age "${go_old}"

go_young="$(repo go-young require_go example.com/young v1.3.0)"
expect_output fail "a Go module two days old" "example.com/young v1.3.0 was published 1 day(s) ago" age "${go_young}"

go_upper="$(repo go-upper require_go github.com/BurntSushi/toml v1.6.0)"
expect_exit pass "a Go module path with capitals, escaped for the proxy" age "${go_upper}"

go_fix="$(repo go-fix young_go_with_advisory)"
expect_output pass "a young Go module listed with its advisory" "GHSA-abcd-1234-wxyz" age "${go_fix}"

go_no_advisory="$(repo go-no-advisory young_go_without_advisory)"
expect_output fail "an exception that names no advisory" "names no advisory" age "${go_no_advisory}"

npm_old="$(repo npm-old resolve_npm @scope/widget 2.0.0)"
expect_exit pass "a scoped npm package a week old" age "${npm_old}"

npm_young="$(repo npm-young resolve_npm left-pad 1.5.0)"
expect_output fail "an npm package two days old" "left-pad 1.5.0" age "${npm_young}"

go_toolchain="$(repo go-toolchain sed -i.bak 's/^go = "1.27.1"/go = "1.27.2"/' mise.toml)"
expect_output fail "a Go toolchain two days old" "go 1.27.2" age "${go_toolchain}"

tools_old="$(repo tools-old mise_tools)"
expect_exit pass "mise tools of each kind a week old" age "${tools_old}"

# A go: pin names the package `go install` builds, and the proxy knows its
# module, a few path elements up.
go_package="$(repo go-package pin_tool go:golang.org/x/vuln/cmd/govulncheck 1.9.0)"
expect_output fail "a Go-built tool whose module is up its package path" \
  "go:golang.org/x/vuln/cmd/govulncheck 1.9.0 was published 1 day(s) ago" age "${go_package}"

pypi_young="$(repo pypi-young pin_tool yamllint 1.39.0)"
expect_output fail "a PyPI tool two days old" "yamllint 1.39.0" age "${pypi_young}"

release_young="$(repo release-young pin_tool jq 1.9.0)"
expect_output fail "a GitHub release two days old, tagged with its name" "jq 1.9.0" age "${release_young}"

unknown="$(repo unknown pin_tool shellcheck 9.9.9)"
expect_output fail "a version whose publish date cannot be learned" "cannot tell when shellcheck 9.9.9 was published" \
  age "${unknown}"

unlisted="$(repo unlisted pin_tool newtool 1.0.0)"
expect_output fail "a mise tool with no known source" "newtool" age "${unlisted}"

untouched="$(repo untouched touch README.md)"
expect_exit pass "a change that adds no version" age "${untouched}"
count_case
if [[ -s "${untouched}.curl" ]]; then
  fail_case "a change that adds no version" "it looked up: $(cat "${untouched}.curl")"
fi

expect_exit 2 "a base git cannot resolve" age "${untouched}" no-such-branch

finish_tests
