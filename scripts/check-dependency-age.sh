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
# build/yamllint/requirements.txt, which Dependabot does not read. It compares
# the versions go.mod, docs/go.mod, web/yarn.lock, mise.toml and
# build/markdownlint/package-lock.json name, and the files
# build/yamllint/requirements.txt pins by their sha256, at HEAD with those at
# the merge base with <base-ref>, and asks where each new one came from when it
# was published: proxy.golang.org for Go modules and the Go toolchain, the npm
# registry, PyPI, nodejs.org, and the GitHub release for every other mise
# tool. A PyPI release takes files for days after its first, so each file is
# dated by its own upload; a requirement pinned by no hash, whose file cannot
# be told, fails, and so does any line of the requirements file but a comment,
# the two options scripts/pin-yamllint.sh writes and a name==version
# requirement pinned by --hash=sha256 alone, an include or a direct URL among
# them. A lock entry is read with the tarball it resolves to and its
# integrity, so a change to either alone is checked as a new pin is: one that
# does not come from the npm registry under its own name and version, by a
# sha512, fails, as one whose sha512 the registry does not list does, and so
# does a lock npm would read other than by its packages map, a lockfileVersion
# 1 among them, or one that cannot be parsed. A mise tool pinned in a
# [tools.<name>] table, whose keys this does not read, fails too. A pin that
# cannot be dated so fails whatever the exceptions file says, which excuses a
# version's age alone: an excepted pin's hash is still looked up.
#
# A version listed in scripts/dependency-age-exceptions.txt with the advisory
# it fixes passes at any age: a known-vulnerability fix overrides the wait. So
# does one listed with a maintainer's dated waiver, waived-YYYY-MM-DD: the
# maintainer may take a version before its week, by name. A waiver dated on no
# real day fails, as an exception naming neither does. A version whose publish
# date cannot be learned fails, since a check that could not look must not
# pass.
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
# version actually turned a week old. A pre-approval no line of the exceptions
# file lists is warned of too, and fails nothing: its line may have gone out
# without it.
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
readonly requirements="build/yamllint/requirements.txt"
readonly npm_lock="build/markdownlint/package-lock.json"
readonly goproxy="https://proxy.golang.org"
readonly npm_registry="https://registry.npmjs.org"
readonly github_api="https://api.github.com"
now="${DEPENDENCY_AGE_NOW:-$(date -u +%s)}"
readonly now

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

# npm_integrity prints the integrity the npm registry lists for a package
# version's tarball.
#   npm_integrity <package> <version>
npm_integrity() {
  local package
  package="$(npm_package "$1")"
  jq -r --arg version "$2" '.versions[$version].dist.integrity // empty' <<<"${package}" 2>/dev/null || true
}

# pypi_release prints a PyPI release's JSON, asked for once a run however many
# of its files are dated.
#   pypi_release <package> <version>
pypi_release() {
  local kept="${answers}/pypi-$1==$2.json"
  if [[ ! -f "${kept}" ]]; then
    "${get[@]}" "https://pypi.org/pypi/$1/$2/json" >"${kept}" 2>/dev/null || true
  fi
  cat "${kept}"
}

# pypi_published prints when the file of a PyPI release with the given sha256
# was uploaded.
#   pypi_published <package> <version> <sha256>
pypi_published() {
  local release
  release="$(pypi_release "$1" "$2")"
  jq -r --arg sha256 "$3" 'first(.urls[] | select(.digests.sha256 == $sha256) | .upload_time_iso_8601) // empty' \
    <<<"${release}" 2>/dev/null || true
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
    *) release_published "$1" "$2" ;;
  esac
}

# published prints when a version of a dependency was published, asking the
# source its ecosystem names; for PyPI, when the file of it with the sha256
# given was. npm-lock is an npm lockfile's entry, dated as npm's.
#   published <go|npm|npm-lock|pypi|mise> <name> <version> [sha256]
published() {
  case "$1" in
    go) go_published "$2" "$3" ;;
    npm | npm-lock) npm_published "$2" "$3" ;;
    pypi) pypi_published "$2" "$3" "${4:-}" ;;
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
# else, a waiver dated on no real day among them.
#   excused <exception>
excused() {
  local day
  day="$(waived_on "$1")"
  if [[ "$1" =~ ^(CVE-[0-9]{4}-[0-9]{4,}|GHSA(-[0-9a-z]{4}){3})$ ]]; then
    echo "fixes $1"
  elif [[ -n "${day}" ]]; then
    echo "waived on ${day} by the maintainer"
  fi
}

# npm_lock_refusal prints why an npm lockfile entry cannot be dated, or
# nothing. npm ci fetches the tarball the entry's resolved names and checks it
# by the entry's integrity alone, so the gate dates only one from the registry
# under its own name and version, by the sha512 the registry lists for it.
#   npm_lock_refusal <name> <version> <resolved> <integrity>
npm_lock_refusal() {
  local listed
  if [[ "$2" == - ]]; then
    echo "names no version in the lock, so it cannot be dated: re-lock it with task deps:markdownlint"
  elif [[ "$3" != "${npm_registry}/$1/-/${1#@*/}-$2.tgz" ]]; then
    echo "does not come from the npm registry under its own name and version (resolved: $3), so it cannot be dated"
  elif [[ "$4" != sha512-* ]]; then
    echo "is pinned by no sha512 integrity, so npm ci would take any tarball: re-lock it with task deps:markdownlint"
  else
    listed="$(npm_integrity "$1" "$2")"
    if [[ "${listed}" != "$4" ]]; then
      echo "is locked by a hash the npm registry does not list for it, so it cannot be dated"
    fi
  fi
}

# refusal prints why a pin cannot be dated whatever the exceptions file says,
# which excuses a version's age alone, or nothing: a requirement pinned by no
# hash or by one PyPI gives no upload for, and an npm lockfile entry
# npm_lock_refusal refuses. So an excepted pin's hash is looked up too.
#   refusal <ecosystem> <name> <version> [sha256 | resolved integrity]
refusal() {
  local uploaded
  case "$1" in
    pypi)
      if [[ -z "${4:-}" ]]; then
        echo "is pinned by no hash, so no file of it can be dated: pin each one with --hash=sha256:<digest>"
        return
      fi
      uploaded="$(pypi_published "$2" "$3" "$4")"
      if [[ -z "${uploaded}" ]]; then
        echo "is pinned by a hash PyPI lists no file of, or PyPI could not be asked, so it cannot be dated"
      fi
      ;;
    npm-lock) npm_lock_refusal "$2" "$3" "${4:-}" "${5:-}" ;;
    *) ;;
  esac
}

# check holds one new version to the age gate, a PyPI file, named by its
# sha256, to its own age, and an npm lockfile entry, by its integrity, to the
# registry's tarball of its version. The exceptions file names a version, so
# it excuses each file of one.
#   check <ecosystem> <name> <version> [sha256 | resolved integrity]
check() {
  local name="$2" version="$3" sha256="" what refused exception reason when epoch
  if [[ "$1" == pypi ]]; then
    sha256="${4:-}"
  fi
  what="${name} ${version}${sha256:+ (sha256:${sha256})}"
  refused="$(refusal "$@")"
  if [[ -n "${refused}" ]]; then
    echo "FAIL  ${what} ${refused}" >&2
    failed=1
    return
  fi

  exception="$(listed_exception "${name}" "${version}")"
  reason="$(excused "${exception}")"
  if [[ -n "${reason}" ]]; then
    echo "ok    ${what}: ${reason}, so it is taken at any age"
    return
  fi
  if [[ -n "${exception}" ]]; then
    echo "FAIL  ${what}: its exception names no advisory or dated waiver (want a CVE or GHSA id, or waived-YYYY-MM-DD)" >&2
    failed=1
    return
  fi

  when="$(published "$1" "${name}" "${version}" "${sha256}")"
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

# refused_file fails a "! why" line a pin file's reader prints in place of its
# pins: requirements_pins quotes the line it refuses, mise_pins the
# [tools.<name>] header, and npm_lock_pins says why it read no pins from the
# lock at all.
#   refused_file <pypi|mise|npm-lock> <why>
refused_file() {
  case "$1" in
    npm-lock) echo "FAIL  ${npm_lock}: $2, so it cannot be dated: re-lock it with task deps:markdownlint" >&2 ;;
    mise) echo "FAIL  mise.toml: '$2' cannot be dated: pin each tool under [tools] as name = \"version\"" >&2 ;;
    *) echo "FAIL  ${requirements}: '$2' cannot be dated: pin by name==version and --hash=sha256 only" >&2 ;;
  esac
  failed=1
}

# check_all checks each "name version" line under one ecosystem, each
# "name version sha256" line naming a PyPI file, and each "name version
# resolved integrity" line naming an npm lockfile entry, and fails each
# "! why" line a pin file's reader prints.
#   check_all <ecosystem> <lines>
check_all() {
  local line name version first second
  while read -r line; do
    if [[ "${line}" == "! "* ]]; then
      refused_file "$1" "${line#! }"
    elif [[ -n "${line}" ]]; then
      read -r name version first second <<<"${line}"
      check "$1" "${name}" "${version}" ${first:+"${first}"} ${second:+"${second}"}
    fi
  done <<<"$2"
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

# warn_of_waiver warns of a waiver past its week on stderr and, in GitHub
# Actions, as an annotation on its line too, which the pull request shows where
# a log line would go unread.
#   warn_of_waiver <line-number> <message>
warn_of_waiver() {
  echo "warn  $2" >&2
  if [[ "${GITHUB_ACTIONS:-}" == true ]]; then
    echo "::warning file=${exceptions},line=$1::$2"
  fi
}

# warn_past_their_week warns of each dated waiver a week past its day, and of
# its version's pre-approval in web/.yarnrc.yml, and fails nothing. A version
# waived on a day was published by that day's end at the latest, so the week
# is counted from then.
warn_past_their_week() {
  local name version exception day start approved where line=0
  if [[ ! -f "${exceptions}" ]]; then
    return
  fi
  approved="$(preapproved)"
  while read -r name version exception _; do
    line=$((line + 1))
    day="$(waived_on "${exception:-}")"
    start="$(day_start "${day}")"
    if [[ -n "${start}" ]] && ((now - start >= day_seconds + min_seconds)); then
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

check_all go "$(added "$(at "${base}" go.mod | go_requires)" "$(at HEAD go.mod | go_requires)")"
check_all go "$(added "$(at "${base}" docs/go.mod | go_requires)" "$(at HEAD docs/go.mod | go_requires)")"
check_all npm "$(added "$(at "${base}" web/yarn.lock | yarn_resolutions)" "$(at HEAD web/yarn.lock | yarn_resolutions)")"
check_all mise "$(added "$(at "${base}" mise.toml | mise_pins)" "$(at HEAD mise.toml | mise_pins)")"
check_all pypi "$(added "$(at "${base}" "${requirements}" | requirements_pins)" \
  "$(at HEAD "${requirements}" | requirements_pins)")"
check_all npm-lock "$(added "$(at "${base}" "${npm_lock}" | npm_lock_pins)" "$(at HEAD "${npm_lock}" | npm_lock_pins)")"
warn_past_their_week
warn_unlisted_preapprovals

exit "${failed}"
