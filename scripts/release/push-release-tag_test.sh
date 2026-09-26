#!/usr/bin/env bash
#
# Tests for push-release-tag.sh. Each case builds a repository with an origin
# and a manifest, makes a head commit, runs the script there with a stand-in for
# gh, and checks whether a tag reached the origin.
#
# Usage:
#   scripts/release/push-release-tag_test.sh
set -euo pipefail

# A git hook or `git rebase --exec` exports the variables that locate its
# repository; the repositories this test builds must not inherit them.
# shellcheck disable=SC2046 # word splitting is the point: one name per word
unset $(git rev-parse --local-env-vars)

script="$(cd "${BASH_SOURCE[0]%/*}" && pwd)/push-release-tag.sh"
readonly script

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# A gh that records what it was asked and answers the one question the script
# puts to it, which pull requests a commit came from, as gh api does: the pulls
# in GH_STUB_PULLS go through the script's own --jq filter, so the filter is
# what decides which of them released the commit.
mkdir -p "${workdir}/bin"
cat >"${workdir}/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${GH_STUB_LOG}"
if [[ "$1" == "api" ]]; then
  while (($# > 0)) && [[ "$1" != "--jq" ]]; do
    shift
  done
  jq -r "${2:-.}" <<<"${GH_STUB_PULLS:-[]}"
fi
STUB
chmod +x "${workdir}/bin/gh"

# The pulls a commit can come from, as gh api lists them.
readonly released='[{"number":42,"merged_at":"2026-09-20T12:00:00Z","labels":[{"name":"autorelease: pending"}]}]'
readonly unlabeled='[{"number":42,"merged_at":"2026-09-20T12:00:00Z","labels":[{"name":"autorelease: tagged"}]}]'
readonly unmerged='[{"number":42,"merged_at":null,"labels":[{"name":"autorelease: pending"}]}]'
readonly no_pulls='[]'

# quiet_git runs git with the developer's own configuration kept out.
quiet_git() {
  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git "$@" >/dev/null 2>&1
}

# repository makes a clone of a fresh origin whose manifest names a version.
#   repository <name> <manifest version>
repository() {
  local name="$1" manifest="$2"
  local origin="${workdir}/${name}.git" clone="${workdir}/${name}"

  quiet_git init --bare --initial-branch=main "${origin}"
  quiet_git init --initial-branch=main "${clone}"
  quiet_git -C "${clone}" config user.name "Test"
  quiet_git -C "${clone}" config user.email "test@example.com"
  quiet_git -C "${clone}" remote add origin "${origin}"
  printf '{\n  ".": "%s"\n}\n' "${manifest}" >"${clone}/.release-please-manifest.json"
  quiet_git -C "${clone}" add .
}

# expect commits a subject, runs the script, and compares what happened.
#   expect <tagged|untagged> <ok|fails> <name> <manifest> <subject> <pulls> [tag]
expect() {
  local want_tag="$1" want_exit="$2" name="$3" manifest="$4" subject="$5" pulls="$6"
  local tag="${7:-v${manifest}}" slug="case${cases}" got_tag="untagged" got_exit="ok"

  cases=$((cases + 1))
  repository "${slug}" "${manifest}"
  quiet_git -C "${workdir}/${slug}" commit -m "${subject}"
  quiet_git -C "${workdir}/${slug}" push origin main

  if ! (cd "${workdir}/${slug}" \
    && PATH="${workdir}/bin:${PATH}" GH_STUB_LOG="${workdir}/${slug}.log" GH_STUB_PULLS="${pulls}" \
      GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 HEAD_COMMIT_MSG="${subject}" \
      bash "${script}" >"${workdir}/${slug}.out" 2>&1); then
    got_exit="fails"
  fi

  # Asked of git directly: a function in a condition runs with set -e off.
  if git -C "${workdir}/${slug}.git" rev-parse --verify --quiet "refs/tags/${tag}" >/dev/null 2>&1; then
    got_tag="tagged"
  fi

  if [[ "${got_tag}" != "${want_tag}" || "${got_exit}" != "${want_exit}" ]]; then
    echo "FAIL ${name}: want ${want_tag} and ${want_exit}, got ${got_tag} and ${got_exit}" >&2
    sed 's/^/    /' "${workdir}/${slug}.out" >&2
    failures=$((failures + 1))
  fi
}

# decides runs the script's decide step on a message and compares the line it
# wrote to GITHUB_OUTPUT, which the workflow's later steps read.
#   decides <name> <message> <want output>
decides() {
  local name="$1" message="$2" want="$3" output="${workdir}/decide${cases}.out" got

  cases=$((cases + 1))
  : >"${output}"
  if ! (cd "${workdir}" \
    && GITHUB_OUTPUT="${output}" HEAD_COMMIT_MSG="${message}" bash "${script}" decide >/dev/null 2>&1); then
    echo "FAIL ${name}: the decide step failed" >&2
    failures=$((failures + 1))
    return
  fi

  got="$(<"${output}")"
  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want '${want}' in GITHUB_OUTPUT, got '${got}'" >&2
    failures=$((failures + 1))
  fi
}

expect untagged ok "an ordinary commit" "1.4.0" "feat(jira): add issue transitions" "${no_pulls}"
expect tagged ok "release-please's own release commit" "1.4.0" "chore(main): release 1.4.0" "${released}"
expect tagged ok "a prerelease" "1.4.0-rc.1" "chore(main): release 1.4.0-rc.1" "${released}"

# The subject is one line anybody can write. What it claims has to be what the
# manifest at that commit says, and the commit has to come from a merged pull
# request release-please labeled as a pending release.
expect untagged fails "a version the manifest does not hold" "1.4.0" "chore(main): release 9.9.9" "${released}" "v9.9.9"
expect untagged fails "no pull request holds the commit" "1.4.0" "chore(main): release 1.4.0" "${no_pulls}"
expect untagged fails "a merged pull request not labeled pending" "1.4.0" "chore(main): release 1.4.0" "${unlabeled}"
expect untagged fails "a labeled pull request not merged" "1.4.0" "chore(main): release 1.4.0" "${unmerged}"
expect untagged ok "words after the version" "1.4.0" "chore(main): release 1.4.0 and more" "${released}"
expect untagged ok "a version that is a path" "1.4.0" "chore(main): release 1.4.0/nested" "${released}" "v1.4.0/nested"

# A tag already on the origin is left alone, and nothing is dispatched twice.
cases=$((cases + 1))
repository again "1.4.0"
quiet_git -C "${workdir}/again" commit -m "chore(main): release 1.4.0"
quiet_git -C "${workdir}/again" push origin main
quiet_git -C "${workdir}/again" tag v1.4.0
quiet_git -C "${workdir}/again" push origin v1.4.0
quiet_git -C "${workdir}/again" tag -d v1.4.0

(cd "${workdir}/again" \
  && PATH="${workdir}/bin:${PATH}" GH_STUB_LOG="${workdir}/again.log" GH_STUB_PULLS="${released}" \
    GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 HEAD_COMMIT_MSG="chore(main): release 1.4.0" \
    bash "${script}" >"${workdir}/again.out" 2>&1) || true

if grep -q 'workflow run' "${workdir}/again.log" 2>/dev/null; then
  echo "FAIL a tag already on the origin: release.yml was dispatched again" >&2
  failures=$((failures + 1))
fi

# A release that goes through dispatches the build and relabels the pull request.
if ! grep -q 'workflow run release.yml --ref v1.4.0' "${workdir}/case1.log" \
  || ! grep -q 'pr edit 42' "${workdir}/case1.log"; then
  echo "FAIL a release: want release.yml dispatched and pull request 42 relabeled, got:" >&2
  sed 's/^/    /' "${workdir}/case1.log" >&2
  failures=$((failures + 1))
fi

decides "decide: a release commit" "chore(main): release 1.4.0" "version=v1.4.0"
decides "decide: a prerelease" "chore(main): release 1.4.0-rc.1" "version=v1.4.0-rc.1"
decides "decide: a release subject above a body" $'chore(main): release 1.4.0\n\nnotes' "version=v1.4.0"
decides "decide: an ordinary commit" "fix: x" "version="
decides "decide: words after the version" "chore(main): release 1.4.0 and more" "version="

# A step name the script does not know is refused rather than read as a request
# to tag, even on a commit that would be tagged.
cases=$((cases + 1))
repository unknown "1.4.0"
quiet_git -C "${workdir}/unknown" commit -m "chore(main): release 1.4.0"
quiet_git -C "${workdir}/unknown" push origin main

if (cd "${workdir}/unknown" \
  && PATH="${workdir}/bin:${PATH}" GH_STUB_LOG="${workdir}/unknown.log" GH_STUB_PULLS="${released}" \
    GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 HEAD_COMMIT_MSG="chore(main): release 1.4.0" \
    bash "${script}" tag >"${workdir}/unknown.out" 2>&1) \
  || git -C "${workdir}/unknown.git" rev-parse --verify --quiet refs/tags/v1.4.0 >/dev/null 2>&1; then
  echo "FAIL an unknown step: want it refused with nothing tagged" >&2
  sed 's/^/    /' "${workdir}/unknown.out" >&2
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  echo "push-release-tag: ${failures} check(s) of ${cases} case(s) failed." >&2
  exit 1
fi

echo "push-release-tag: all ${cases} cases pass."
