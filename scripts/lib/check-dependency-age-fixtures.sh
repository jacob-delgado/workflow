# shellcheck shell=bash
# shellcheck disable=SC2154 # here and workdir are the sourcing test's
#
# The fixtures check-dependency-age.sh's two tests build their cases from: a
# stand-in curl that answers each registry URL from a fixture file named for
# it, logs it, and fails as curl -f does for any other, so no case reaches the
# network; the registries' answers the two tests share, for Go modules, npm
# packages, node, and mise tools' GitHub releases, a version two days old or a
# week old; a repository whose main branch holds a go.mod, a web/yarn.lock and
# a mise.toml and whose branch makes a case's change; and the check run in it
# against main, outside GitHub Actions or as a job there. DEPENDENCY_AGE_NOW
# fixes "now" at 2026-10-08T00:00:00Z. check-dependency-age_test.sh drives
# go.mod, web/yarn.lock, mise.toml, the exceptions file and its warnings;
# check-dependency-age_pins_test.sh the linters' pin files,
# build/yamllint/requirements.txt and build/markdownlint/package-lock.json.
#
# Sourced, never run, after lib/testing.sh, which sets ${workdir}; check is the
# gate beside the sourcing test's ${here}:
#
#   # shellcheck source=lib/check-dependency-age-fixtures.sh
#   source "${here}/lib/check-dependency-age-fixtures.sh"

readonly check="${here}/check-dependency-age.sh"

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

# integrity_of prints the sha512 integrity the npm registry lists for a
# package version: a stand-in, though spelled as npm spells one.
#   integrity_of <package> <version>
integrity_of() {
  printf 'sha512-%s' "$(printf '%s@%s' "$1" "$2" | base64 | tr -d '\n')"
}

answer "https://proxy.golang.org/example.com/old/@v/v1.2.0.info" "{\"Version\":\"v1.2.0\",\"Time\":\"${old}\"}"
answer "https://proxy.golang.org/example.com/young/@v/v1.3.0.info" "{\"Version\":\"v1.3.0\",\"Time\":\"${young}\"}"
answer "https://proxy.golang.org/github.com/!burnt!sushi/toml/@v/v1.6.0.info" "{\"Version\":\"v1.6.0\",\"Time\":\"${old}\"}"
answer "https://registry.npmjs.org/left-pad" "{\"time\":{\"1.4.0\":\"${old}\",\"1.5.0\":\"${young}\"}}"
answer "https://registry.npmjs.org/@scope%2Fwidget" "{\"time\":{\"2.0.0\":\"${old}\"},
  \"versions\":{\"2.0.0\":{\"dist\":{\"integrity\":\"$(integrity_of @scope/widget 2.0.0)\"}}}}"
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
  repo_on "$1" : "${@:2}"
}

# repo_on makes a repository as repo does, its main branch changed first by a
# command of its own.
#   repo_on <name> <base-command> <change-command> [arg...]
repo_on() {
  local dir="${workdir}/$1" base="$2"
  shift 2
  mkdir -p "${dir}/web" "${dir}/scripts"
  git -C "${dir}" init -q --initial-branch=main
  printf 'module example\n\ngo 1.27\n\nrequire (\n\texample.com/kept v1.0.0\n)\n' >"${dir}/go.mod"
  printf '"kept@npm:^1.0.0":\n  version: 1.0.0\n  resolution: "kept@npm:1.0.0"\n' >"${dir}/web/yarn.lock"
  printf 'min_version = "2026.9.3"\n\n[tools]\ngo = "1.27.1"\n' >"${dir}/mise.toml"
  printf '# advisories\n' >"${dir}/scripts/dependency-age-exceptions.txt"
  (cd "${dir}" && "${base}")
  git -C "${dir}" add -A
  git -C "${dir}" -c user.email=t@example.com -c user.name=t commit -q -m base
  git -C "${dir}" switch -q -c change
  (cd "${dir}" && "$@")
  git -C "${dir}" add -A
  git -C "${dir}" -c user.email=t@example.com -c user.name=t commit -q -m change
  printf '%s' "${dir}"
}

# except lists a version in the exceptions file.
except() {
  printf '%s\n' "$*" >>scripts/dependency-age-exceptions.txt
}

# age runs the check in a repository against main, with the stand-in curl,
# outside GitHub Actions whatever runs the test.
#   age <dir> [base]
age() {
  : >"$1.curl"
  (cd "$1" && env -u GITHUB_ACTIONS PATH="${stub_bin}:${PATH}" CURL_STUB_LOG="$1.curl" \
    CURL_STUB_FIXTURES="${fixtures}" DEPENDENCY_AGE_NOW="${now}" "${check}" "${2:-main}")
}

# age_in_actions runs age as a GitHub Actions job does.
#   age_in_actions <dir>
age_in_actions() {
  : >"$1.curl"
  (cd "$1" && env PATH="${stub_bin}:${PATH}" CURL_STUB_LOG="$1.curl" CURL_STUB_FIXTURES="${fixtures}" \
    DEPENDENCY_AGE_NOW="${now}" GITHUB_ACTIONS=true "${check}" main)
}
