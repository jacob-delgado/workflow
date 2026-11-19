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
# rest — a hand-run `go get` or `yarn add`, and mise.toml and
# docs/media/tool-checksums.txt, which Dependabot does not read. It compares
# the versions go.mod, docs/go.mod, web/yarn.lock, mise.toml and
# docs/media/tool-checksums.txt (the docs' ttyd) name, a go.mod's replace
# directives among them, at HEAD with those at the merge base with
# <base-ref>, and asks where each new one came from when it was published:
# proxy.golang.org for Go modules and the Go toolchain, the npm registry,
# nodejs.org, and the GitHub release for every other mise tool and the docs'
# ttyd. A mise tool is read from its version string or an inline table's
# version key, under a [tools] header however it is spaced, quoted or
# commented; one pinned in an inline table naming no version, in a
# [tools.<name>] table, or by a tools key before any table, whose keys this
# does not read, fails, whatever the exceptions file says.
#
# A version listed in scripts/dependency-age-exceptions.txt with the advisory
# it fixes passes at any age: a known-vulnerability fix overrides the wait. So
# does one listed with a maintainer's dated waiver, waived-YYYY-MM-DD: the
# maintainer may take a version before its week, by name. A waiver dated on no
# real day fails, as an exception naming neither does, and so does one dated on
# a day still to come wherever the maintainer is, more than a day after now's
# in UTC: such a waiver would excuse its version at any age and never come due.
# A version whose publish date cannot be learned fails, since a check that
# could not look must not pass.
#
# A waiver has done its job once its version is a week old, and its line is
# then due out, with the version's npmPreapprovedPackages entry in
# web/.yarnrc.yml where yarn's own gate needed one. A version is published no
# later than the day it is waived, so a week after that day ends, it is surely
# a week old: from then, every run warns of the waiver, whatever the change
# adds, and fails nothing for it; in GitHub Actions the warning is also a
# ::warning annotation on the waiver's line, so the pull request shows it. The
# week is counted from that day, with nothing looked up, so the warning is
# never early, and can come up to a week, or a day more at most, after the
# version actually turned a week old. A waiver dated on a day still to come is
# warned of too, since it excuses nothing. A pre-approval no line of the
# exceptions file lists is warned of too, and fails nothing: its line may have
# gone out without it.
#
# Exits 0 when every new version is old enough, 1 when one is not or cannot be
# dated, and 2 when git cannot compare with <base-ref>.
set -euo pipefail

if (($# != 1)); then
  sed -n '7,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

readonly day_seconds=$((24 * 60 * 60))
readonly min_seconds=$((7 * day_seconds))
readonly exceptions="scripts/dependency-age-exceptions.txt"
readonly yarnrc="web/.yarnrc.yml"
readonly media_checksums="docs/media/tool-checksums.txt"
readonly goproxy="https://proxy.golang.org"
readonly npm_registry="https://registry.npmjs.org"
readonly github_api="https://api.github.com"
now="${DEPENDENCY_AGE_NOW:-$(date -u +%s)}"
readonly now
# The latest second a waiver's day may start: the day after now's in UTC is
# already today east of it, and any later one is still to come wherever the
# maintainer is.
readonly last_waiver_start=$((now + day_seconds))

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/dependency-pins.sh
source "${here}/lib/dependency-pins.sh"

if ! base="$(git merge-base "$1" HEAD 2>/dev/null)"; then
  echo "check-dependency-age: cannot compare with $1" >&2
  exit 2
fi
readonly base

failed=0
readonly -a get=(curl -fsSL --retry 2)
# The registries' answers, each asked for once a run: every lookup runs in a
# subshell of its own, so an answer is kept in a file here.
answers="$(mktemp -d)"
readonly answers
trap 'rm -rf "${answers}"' EXIT

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

# npm_package prints an npm package's registry document, asked for once a run
# however many of its versions are dated or checked.
#   npm_package <package>
npm_package() {
  local kept="${answers}/npm-${1/\//%2F}.json"
  if [[ ! -f "${kept}" ]]; then
    "${get[@]}" "${npm_registry}/${1/\//%2F}" >"${kept}" 2>/dev/null || true
  fi
  cat "${kept}"
}

# npm_published prints when an npm package version was published.
#   npm_published <package> <version>
npm_published() {
  local package
  package="$(npm_package "$1")"
  jq -r --arg version "$2" '.time[$version] // empty' <<<"${package}" 2>/dev/null || true
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
    rumdl) echo rvben/rumdl ;;
    jq) echo jqlang/jq ;;
    hugo-extended) echo gohugoio/hugo ;;
    air) echo air-verse/air ;;
    vhs) echo charmbracelet/vhs ;;
    ttyd) echo tsl0922/ttyd ;;
    *) ;;
  esac
}

# release_published prints when a tool's GitHub release was published, trying
# the tag spellings releases use: v1.2.3, 1.2.3 and tool-1.2.3, and nothing
# for a tool release_repository knows no repository of.
#   release_published <tool> <version>
release_published() {
  local repository tag published=""
  local -a auth=()
  repository="$(release_repository "$1")"
  if [[ -z "${repository}" ]]; then
    return
  fi
  if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    auth=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  fi
  for tag in "v$2" "$2" "$1-$2"; do
    if [[ -z "${published}" ]]; then
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

# listed_exception prints what the exceptions file lists a version under, an
# advisory or a waiver, "none" for a line that names neither, or nothing when
# no line lists it.
#   listed_exception <name> <version>
listed_exception() {
  if [[ -f "${exceptions}" ]]; then
    awk -v name="$1" -v version="$2" '$1 == name && $2 == version { print ($3 == "" ? "none" : $3); exit }' \
      "${exceptions}"
  fi
}

# waived_on prints the day a dated waiver names, and nothing for an exception
# that is no waiver or a waiver dated on no real day: a day the calendar lacks,
# such as February 30, does not read back as itself.
#   waived_on <exception>
waived_on() {
  if [[ "$1" =~ ^waived-([0-9]{4}-[0-9]{2}-[0-9]{2})$ ]]; then
    jq -rn --arg day "${BASH_REMATCH[1]}" '($day + "T00:00:00Z") as $time
      | if ($time | fromdateiso8601 | todate) == $time then $day else empty end' \
      2>/dev/null || true
  fi
}

# excused prints why an exception lets a version skip the wait: the advisory it
# fixes, or the day the maintainer waived it. It prints nothing for anything
# else, a waiver dated on no real day or on a day still to come among them.
#   excused <exception>
excused() {
  local day start
  day="$(waived_on "$1")"
  start="$(day_start "${day}")"
  if [[ "$1" =~ ^(CVE-[0-9]{4}-[0-9]{4,}|GHSA(-[0-9a-z]{4}){3})$ ]]; then
    echo "fixes $1"
  elif [[ -n "${start}" ]] && ((start <= last_waiver_start)); then
    echo "waived on ${day} by the maintainer"
  fi
}

# check holds one new version to the age gate.
#   check <ecosystem> <name> <version>
check() {
  local name="$2" version="$3" what exception reason when epoch
  what="${name} ${version}"
  exception="$(listed_exception "${name}" "${version}")"
  reason="$(excused "${exception}")"
  if [[ -n "${reason}" ]]; then
    echo "ok    ${what}: ${reason}, so it is taken at any age"
    return
  fi
  if [[ -n "${exception}" ]]; then
    echo "FAIL  ${what}: its exception names no advisory or dated waiver (want a CVE or GHSA id, or waived-YYYY-MM-DD on a real day no later than today)" >&2
    failed=1
    return
  fi

  when="$(published "$1" "${name}" "${version}")"
  epoch="$(jq -rn --arg time "${when}" '$time | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601' 2>/dev/null || true)"
  if [[ -z "${when}" || -z "${epoch}" ]]; then
    echo "FAIL  cannot tell when ${what} was published; a version that cannot be dated is not adopted" >&2
    failed=1
  elif ((now - epoch < min_seconds)); then
    echo "FAIL  ${what} was published $(((now - epoch) / 86400)) day(s) ago, on ${when%%T*}: wait until it is 7 days old, or list it in ${exceptions} with the advisory it fixes or the maintainer's dated waiver" >&2
    failed=1
  else
    echo "ok    ${what}: published ${when%%T*}"
  fi
}

# refused_file fails a "! line" mise_pins prints in place of a pin it cannot
# read a version from, quoting the line.
#   refused_file <line>
refused_file() {
  echo "FAIL  mise.toml: '$1' cannot be dated: pin each tool under [tools] as name = \"version\" or name = { version = \"version\", ... }" >&2
  failed=1
}

# check_all checks each "name version" line under one ecosystem, and fails
# each "! line" mise_pins prints.
#   check_all <ecosystem> <lines>
check_all() {
  local line name version
  while read -r line; do
    if [[ "${line}" == "! "* ]]; then
      refused_file "${line#! }"
    elif [[ -n "${line}" ]]; then
      read -r name version <<<"${line}"
      check "$1" "${name}" "${version}"
    fi
  done <<<"$2"
}

# media_pins prints "tool version" once for each version the docs' checksums
# file pins, which holds a line a machine: ttyd is pinned there, not in
# mise.toml, and dated as a mise tool's GitHub release is.
media_pins() {
  awk '!/^#/ && NF >= 2 && !seen[$1 " " $2]++ { print $1, $2 }'
}

# preapproved prints "package version" for each npmPreapprovedPackages entry
# in web/.yarnrc.yml, which spells one 'package@version': the last @ parts
# them, as a scope starts with one.
preapproved() {
  if [[ -f "${yarnrc}" ]]; then
    awk -v quote="'" '
      /^[^ \t#]/ { listed = ($0 ~ /^npmPreapprovedPackages:/); next }
      listed && /^[ \t]*- / {
        entry = $0
        sub(/^[ \t]*- /, "", entry)
        sub(/[ \t]+#.*$/, "", entry)
        gsub(quote, "", entry)
        gsub(/"/, "", entry)
        at = match(entry, /@[^@]*$/)
        if (at > 1) { print substr(entry, 1, at - 1), substr(entry, at + 1) }
      }
    ' "${yarnrc}"
  fi
}

# day_start prints the second a day starts, UTC, and nothing for no day.
#   day_start <YYYY-MM-DD>
day_start() {
  if [[ -n "$1" ]]; then
    jq -rn --arg day "$1" '$day + "T00:00:00Z" | fromdateiso8601' 2>/dev/null || true
  fi
}

# warn_of_waiver warns of a waiver on stderr and, in GitHub Actions, as an
# annotation on its line too, which the pull request shows where a log line
# would go unread.
#   warn_of_waiver <line-number> <message>
warn_of_waiver() {
  echo "warn  $2" >&2
  if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
    echo "::warning file=${exceptions},line=$1::$2"
  fi
}

# warn_of_waivers warns of each dated waiver a week past its day, and of its
# version's pre-approval in web/.yarnrc.yml, and of each dated on a day still
# to come, which excuses nothing, and fails nothing. A version waived on a day
# was published by that day's end at the latest, so the week is counted from
# then.
warn_of_waivers() {
  local name version exception day start approved where line=0
  if [[ ! -f "${exceptions}" ]]; then
    return
  fi
  approved="$(preapproved)"
  while read -r name version exception _; do
    line=$((line + 1))
    day="$(waived_on "${exception:-}")"
    start="$(day_start "${day}")"
    if [[ -n "${start}" ]] && ((start > last_waiver_start)); then
      warn_of_waiver "${line}" "${name} ${version}: waived on ${day}, a day still to come, so it excuses nothing; date it on the day the maintainer waived it"
    elif [[ -n "${start}" ]] && ((now - start >= day_seconds + min_seconds)); then
      where="its line from ${exceptions}"
      if grep -qxF -- "${name} ${version}" <<<"${approved}"; then
        where="${where} and its npmPreapprovedPackages entry from ${yarnrc}"
      fi
      warn_of_waiver "${line}" "${name} ${version}: waived on ${day}, so a week old by now; remove ${where}"
    fi
  done <"${exceptions}"
}

# warn_unlisted_preapprovals warns of each npmPreapprovedPackages entry in
# web/.yarnrc.yml that no line of the exceptions file lists, and fails
# nothing: the exceptions file says why a version is let in early, and an entry
# whose line has gone, with its week, is due out too.
warn_unlisted_preapprovals() {
  local name version listed
  while read -r name version; do
    listed="$(listed_exception "${name}" "${version}")"
    if [[ -z "${listed}" ]]; then
      echo "warn  ${name} ${version}: ${yarnrc} pre-approves it, but no line of ${exceptions} says why; remove its npmPreapprovedPackages entry, or list it there with its advisory or dated waiver" >&2
    fi
  done < <(preapproved)
}

check_all go "$(added "$(at "${base}" go.mod | go_module_pins)" "$(at HEAD go.mod | go_module_pins)")"
check_all go "$(added "$(at "${base}" docs/go.mod | go_module_pins)" "$(at HEAD docs/go.mod | go_module_pins)")"
check_all npm "$(added "$(at "${base}" web/yarn.lock | yarn_resolutions)" "$(at HEAD web/yarn.lock | yarn_resolutions)")"
check_all mise "$(added "$(at "${base}" mise.toml | mise_pins)" "$(at HEAD mise.toml | mise_pins)")"
check_all mise "$(added "$(at "${base}" "${media_checksums}" | media_pins)" \
  "$(at HEAD "${media_checksums}" | media_pins)")"
warn_of_waivers
warn_unlisted_preapprovals

exit "${failed}"
