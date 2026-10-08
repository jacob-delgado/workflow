#!/usr/bin/env bash
#
# Fail when a change adopts a dependency version published less than seven
# days before.
#
# Usage:
#   scripts/check-dependency-age.sh <base-ref>
#
# CLAUDE.md's age gate: a freshly compromised release is usually caught and
# yanked within days, so no version is adopted before it is a week old.
# Dependabot's cooldown holds its own pull requests to that; this holds the
# rest — a hand-run `go get` or `yarn add`, and mise.toml, which Dependabot
# does not read. It compares the versions go.mod, docs/go.mod, web/yarn.lock
# and mise.toml name at HEAD with those at the merge base with <base-ref>, and
# asks where each new one came from when it was published: proxy.golang.org
# for Go modules and the Go toolchain, the npm registry, PyPI, nodejs.org, and
# the GitHub release for every other mise tool.
#
# A version listed in scripts/dependency-age-exceptions.txt with the advisory
# it fixes passes at any age: a known-vulnerability fix overrides the wait. A
# version whose publish date cannot be learned fails, since a check that could
# not look must not pass.
#
# Exits 0 when every new version is old enough, 1 when one is not or cannot be
# dated, and 2 when git cannot compare with <base-ref>.
set -euo pipefail

if (($# != 1)); then
  sed -n '7,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

readonly min_seconds=$((7 * 24 * 60 * 60))
readonly exceptions="scripts/dependency-age-exceptions.txt"
readonly goproxy="https://proxy.golang.org"
readonly npm_registry="https://registry.npmjs.org"
readonly github_api="https://api.github.com"
now="${DEPENDENCY_AGE_NOW:-$(date -u +%s)}"
readonly now

if ! base="$(git merge-base "$1" HEAD 2>/dev/null)"; then
  echo "check-dependency-age: cannot compare with $1" >&2
  exit 2
fi
readonly base

failed=0
readonly -a get=(curl -fsSL --retry 2)

# Every lookup below prints what it found or nothing, and never fails: set -e
# does not reach a function called inside a condition, so none is called there.

# at prints a file as it stands at a revision, or nothing where it is absent.
at() {
  git show "$1:$2" 2>/dev/null || true
}

# added prints the lines of the second list the first does not hold.
added() {
  comm -13 <(sort -u <<<"$1") <(sort -u <<<"$2") | grep . || true
}

# go_requires prints "module version" for each requirement in a go.mod.
go_requires() {
  awk '
    /^require[ \t]*\(/ { block = 1; next }
    block && /^\)/ { block = 0; next }
    block && NF >= 2 && $1 !~ /^\/\// { print $1, $2; next }
    /^require[ \t]+[^ \t(]/ { print $2, $3 }
  '
}

# yarn_resolutions prints "package version" for each npm package a yarn.lock
# resolves.
yarn_resolutions() {
  sed -n -E 's/^  resolution: "(.+)@npm:([0-9][^"]*)"$/\1 \2/p'
}

# mise_pins prints "tool version" for each [tools] pin in a mise.toml, and for
# its min_version as the tool "mise".
mise_pins() {
  awk '
    function value(line) { sub(/^[^=]*=[ \t]*"/, "", line); sub(/".*/, "", line); return line }
    /^min_version[ \t]*=/ { print "mise", value($0); next }
    /^\[/ { tools = ($0 == "[tools]"); next }
    tools && /^[^#].*=/ { key = $0; sub(/[ \t]*=.*/, "", key); gsub(/"/, "", key); print key, value($0) }
  '
}

# escape_module spells a module path as the Go proxy wants it: each capital
# as "!" and its lower case.
escape_module() {
  awk '{ out = ""; for (i = 1; i <= length($0); i++) { c = substr($0, i, 1); out = out (c ~ /[A-Z]/ ? "!" tolower(c) : c) } print out }' <<<"$1"
}

# go_published prints when a Go module version reached the proxy.
#   go_published <module> <version>
go_published() {
  local module
  module="$(escape_module "$1")"
  "${get[@]}" "${goproxy}/${module}/@v/$2.info" 2>/dev/null | jq -r '.Time // empty' 2>/dev/null || true
}

# package_published prints when the module holding a Go package published a
# version: a mise go: pin names the package `go install` builds, so this asks
# the proxy for each shorter path in turn until one is a module it knows.
#   package_published <package> <version>
package_published() {
  local path="$1" published=""
  while [[ -z "${published}" && "${path}" == */* ]]; do
    published="$(go_published "${path}" "$2")"
    path="${path%/*}"
  done
  printf '%s' "${published}"
}

# npm_published prints when an npm package version was published.
#   npm_published <package> <version>
npm_published() {
  "${get[@]}" "${npm_registry}/${1/\//%2F}" 2>/dev/null \
    | jq -r --arg version "$2" '.time[$version] // empty' 2>/dev/null || true
}

# release_repository prints the GitHub repository a mise tool is released
# from, or nothing for a tool this check has not been told about.
release_repository() {
  case "$1" in
    mise) echo jdx/mise ;;
    task) echo go-task/task ;;
    golangci-lint) echo golangci/golangci-lint ;;
    lefthook) echo evilmartians/lefthook ;;
    shellcheck) echo koalaman/shellcheck ;;
    shfmt) echo mvdan/sh ;;
    typos) echo crate-ci/typos ;;
    actionlint) echo rhysd/actionlint ;;
    hadolint) echo hadolint/hadolint ;;
    taplo) echo tamasfe/taplo ;;
    zizmor) echo zizmorcore/zizmor ;;
    jq) echo jqlang/jq ;;
    hugo-extended) echo gohugoio/hugo ;;
    air) echo air-verse/air ;;
    *) ;;
  esac
}

# release_published prints when a tool's GitHub release was published, trying
# the tag spellings releases use: v1.2.3, 1.2.3 and tool-1.2.3.
#   release_published <tool> <version>
release_published() {
  local repository tag published=""
  local -a auth=()
  repository="$(release_repository "$1")"
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    auth=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  for tag in "v$2" "$2" "$1-$2"; do
    if [[ -n "${repository}" && -z "${published}" ]]; then
      published="$("${get[@]}" ${auth[@]+"${auth[@]}"} "${github_api}/repos/${repository}/releases/tags/${tag}" \
        2>/dev/null | jq -r '.published_at // empty' 2>/dev/null || true)"
    fi
  done
  printf '%s' "${published}"
}

# mise_published prints when a mise.toml pin's version was published.
#   mise_published <tool> <version>
mise_published() {
  case "$1" in
    go) go_published golang.org/toolchain "v0.0.1-go$2.linux-amd64" ;;
    go:*) package_published "${1#go:}" "v${2#v}" ;;
    npm:*) npm_published "${1#npm:}" "$2" ;;
    node)
      "${get[@]}" "https://nodejs.org/dist/index.json" 2>/dev/null \
        | jq -r --arg version "v$2" '.[] | select(.version == $version) | .date + "T00:00:00Z"' 2>/dev/null || true
      ;;
    yamllint)
      "${get[@]}" "https://pypi.org/pypi/yamllint/$2/json" 2>/dev/null \
        | jq -r '[.urls[].upload_time_iso_8601] | min // empty' 2>/dev/null || true
      ;;
    *) release_published "$1" "$2" ;;
  esac
}

# published prints when a version of a dependency was published, asking the
# source its ecosystem names.
#   published <go|npm|mise> <name> <version>
published() {
  case "$1" in
    go) go_published "$2" "$3" ;;
    npm) npm_published "$2" "$3" ;;
    mise) mise_published "$2" "$3" ;;
    *) ;;
  esac
}

# advisory prints the advisory the exceptions file lists a version under, or
# nothing.
#   advisory <name> <version>
advisory() {
  if [[ -f "${exceptions}" ]]; then
    awk -v name="$1" -v version="$2" '$1 == name && $2 == version { print ($3 == "" ? "none" : $3); exit }' \
      "${exceptions}"
  fi
}

# check holds one new version to the age gate.
#   check <ecosystem> <name> <version>
check() {
  local name="$2" version="$3" fix when epoch
  fix="$(advisory "${name}" "${version}")"
  if [[ "${fix}" =~ ^(CVE-[0-9]{4}-[0-9]{4,}|GHSA(-[0-9a-z]{4}){3})$ ]]; then
    echo "ok    ${name} ${version}: fixes ${fix}, so it is taken at any age"
    return
  fi
  if [[ -n "${fix}" ]]; then
    echo "FAIL  ${name} ${version}: its exception names no advisory (want a CVE or GHSA id)" >&2
    failed=1
    return
  fi

  when="$(published "$1" "${name}" "${version}")"
  epoch="$(jq -rn --arg time "${when}" '$time | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601' 2>/dev/null || true)"
  if [[ -z "${when}" || -z "${epoch}" ]]; then
    echo "FAIL  cannot tell when ${name} ${version} was published; a version that cannot be dated is not adopted" >&2
    failed=1
  elif ((now - epoch < min_seconds)); then
    echo "FAIL  ${name} ${version} was published $(((now - epoch) / 86400)) day(s) ago, on ${when%%T*}: wait until it is 7 days old, or list it in ${exceptions} with the advisory it fixes" >&2
    failed=1
  else
    echo "ok    ${name} ${version}: published ${when%%T*}"
  fi
}

# check_all checks each "name version" line under one ecosystem.
#   check_all <ecosystem> <lines>
check_all() {
  local name version
  while read -r name version; do
    if [[ -n "${name}" ]]; then
      check "$1" "${name}" "${version}"
    fi
  done <<<"$2"
}

check_all go "$(added "$(at "${base}" go.mod | go_requires)" "$(at HEAD go.mod | go_requires)")"
check_all go "$(added "$(at "${base}" docs/go.mod | go_requires)" "$(at HEAD docs/go.mod | go_requires)")"
check_all npm "$(added "$(at "${base}" web/yarn.lock | yarn_resolutions)" "$(at HEAD web/yarn.lock | yarn_resolutions)")"
check_all mise "$(added "$(at "${base}" mise.toml | mise_pins)" "$(at HEAD mise.toml | mise_pins)")"

exit "${failed}"
