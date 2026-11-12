#!/usr/bin/env bash
# Push the vX.Y.Z tag for a merged release-please PR, then fire release.yml.
#
# Usage:
#   push-release-tag.sh decide   write version=vX.Y.Z to GITHUB_OUTPUT for a
#                                release commit, and version= for any other
#   push-release-tag.sh          tag the head commit, if it is a release
#
# Runs from .github/workflows/release-please.yml on every push to main: first as
# decide, whose answer the workflow reads to choose whether to provision, gate
# and tag, then, for a release commit, as the tag step. The subject rule below is
# the one statement of what a release commit looks like.
#
# Why this exists at all: release-please is configured with
# `skip-github-release: true`, because GitHub treats a PUBLISHED release as
# immutable for asset uploads — if release-please creates and publishes the
# release, the build workflow cannot attach binaries to it afterwards. Skipping
# release creation fixes that, but release-please bundles TAG creation with
# release creation, so both get skipped and no tag ever appears. This script is
# the missing half: it tags the merge commit, and release.yml creates the release
# with its assets in one atomic call.
#
# It also calls `gh workflow run` explicitly rather than relying on the tag push
# to trigger release.yml. A ref pushed with GITHUB_TOKEN is authored by
# github-actions[bot], and GitHub's anti-loop guard suppresses workflow triggers
# from that identity — so `push: tags` would never fire. The workflow_dispatch
# path has no such guard, which is what lets this work without a maintainer PAT.
#
# What makes a commit a release: a subject is one line that anybody can write, so
# it only opens the question. The version it names has to be the one the
# manifest holds at that commit, and the commit has to come from the pull request
# release-please itself labeled as a pending release. Anything less stops here,
# failing the job where a maintainer will see it, because a published release
# cannot be taken back.
#
# A rerun picks up where a run was cut off. The tag alone does not say the
# release is under way: a run can push it and then fail, or be canceled, before
# it dispatches release.yml, and that tag, pushed with GITHUB_TOKEN, starts no
# build of its own. So a tag already on the remote is done with only once
# release.yml has a run for it; until then the rerun checks the commit as the
# first run did and dispatches the build and flips the label itself. A run
# whose relabel GitHub refuses fails, since release-please offers no next
# release while a merged pull request is labeled pending, and its rerun, with
# the build under way, flips the label of the pending pull request that holds
# the commit, if one still does.
#
# Environment:
#   HEAD_COMMIT_MSG  the pushed commit's message, passed as an env var rather
#                    than interpolated into the workflow's run: line, because its
#                    content is attacker-influenceable.
#   GITHUB_OUTPUT    the file the decide step appends its answer to.
#   GH_TOKEN         needs actions:write (workflow run, and run list for a tag
#                    already pushed) and pull-requests:write (label flip); only
#                    the tag step reads it.
set -euo pipefail

readonly manifest=".release-please-manifest.json"
readonly pending_label="autorelease: pending"
readonly head_commit_msg="${HEAD_COMMIT_MSG:?HEAD_COMMIT_MSG is required}"

subject="$(printf '%s\n' "${head_commit_msg}" | head -n1)"

# release-please's merge commit subject, e.g. "chore(main): release 1.4.0": a
# version and nothing after it, so nothing but a version can become a tag name.
version="$(printf '%s\n' "${subject}" \
  | sed -n -E 's/^chore\(main\): release ([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?)$/\1/p')"

case "${1:-}" in
  decide)
    printf 'version=%s\n' "${version:+v${version}}" >>"${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
    exit 0
    ;;
  "") ;;
  *)
    echo "Unknown step '$1'; the only one is decide." >&2
    exit 2
    ;;
esac

if [[ -z "${version}" ]]; then
  echo "Not a release-please release commit; nothing to tag."
  echo "  subject: ${subject}"
  exit 0
fi

readonly tag="v${version}"

# pending_pull prints the number of the pull request a commit on main came
# from, rebased or not, if release-please labeled it pending, and nothing
# otherwise.
pending_pull() {
  gh api "repos/{owner}/{repo}/commits/$(git rev-parse HEAD)/pulls" \
    --jq "[.[] | select(.merged_at != null) | select(any(.labels[]; .name == \"${pending_label}\")) | .number] | first // empty"
}

# relabel flips release-please's own bookkeeping label on a pull request, so
# its next run sees this release as handled rather than still pending.
#   relabel <pull request number>
relabel() {
  echo "Marking PR #$1 as tagged"
  if ! gh pr edit "$1" --remove-label "${pending_label}" --add-label 'autorelease: tagged'; then
    echo "Could not relabel PR #$1, so release-please offers no next release; rerun this job." >&2
    exit 1
  fi
}

tag_pushed=false
if git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null 2>&1; then
  tag_pushed=true
  # Any run, dispatched or started by a tag pushed by hand, whatever its result:
  # one that failed is the maintainer's to rerun, not a reason to start another.
  release_runs="$(gh run list --workflow release.yml --branch "${tag}" --limit 1 --json databaseId --jq 'length')"
  if [[ "${release_runs}" != "0" ]]; then
    pr_number="$(pending_pull)"
    if [[ -n "${pr_number}" ]]; then
      relabel "${pr_number}"
    fi
    echo "Tag ${tag} is on the remote and release.yml has run for it; nothing more to do."
    exit 0
  fi
fi

manifest_version="$(sed -n -E 's/.*"\."[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "${manifest}")"

if [[ "${manifest_version}" != "${version}" ]]; then
  echo "The subject names ${version} and ${manifest} holds '${manifest_version}'; not releasing." >&2
  exit 1
fi

pr_number="$(pending_pull)"

if [[ -z "${pr_number}" ]]; then
  echo "No merged pull request labeled '${pending_label}' holds this commit; not releasing." >&2
  exit 1
fi

if [[ "${tag_pushed}" == true ]]; then
  echo "Tag ${tag} is on the remote with no run of release.yml; finishing the release of pull request #${pr_number}"
else
  echo "Tagging $(git rev-parse --short HEAD) as ${tag}, released by pull request #${pr_number}"
  git tag "${tag}"
  git push origin "${tag}"
fi

echo "Dispatching release.yml for ${tag}"
gh workflow run release.yml --ref "${tag}"

# The release is already on its way, so a refused relabel fails the run
# without undoing it, and the rerun flips the label.
relabel "${pr_number}"
