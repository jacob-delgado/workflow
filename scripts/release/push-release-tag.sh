#!/usr/bin/env bash
# Push the vX.Y.Z tag for a merged release-please PR, then fire release.yml.
#
# Runs from .github/workflows/release-please.yml on every push to main; it exits
# quietly unless the head commit is release-please's own release commit.
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
# Environment:
#   HEAD_COMMIT_MSG  the pushed commit's message, passed as an env var rather
#                    than interpolated into the workflow's run: line, because its
#                    content is attacker-influenceable.
#   GH_TOKEN         needs actions:write (workflow run) and pull-requests:write
#                    (label flip).
set -euo pipefail

readonly manifest=".release-please-manifest.json"
readonly pending_label="autorelease: pending"
readonly head_commit_msg="${HEAD_COMMIT_MSG:?HEAD_COMMIT_MSG is required}"

subject="$(printf '%s\n' "${head_commit_msg}" | head -n1)"

# release-please's merge commit subject, e.g. "chore(main): release 1.4.0": a
# version and nothing after it, so nothing but a version can become a tag name.
version="$(printf '%s\n' "${subject}" \
  | sed -n -E 's/^chore\(main\): release ([0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?)$/\1/p')"

if [[ -z "${version}" ]]; then
  echo "Not a release-please release commit; nothing to tag."
  echo "  subject: ${subject}"
  exit 0
fi

readonly tag="v${version}"

if git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null 2>&1; then
  echo "Tag ${tag} already exists on the remote; nothing to do."
  exit 0
fi

released="$(sed -n -E 's/.*"\."[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "${manifest}")"

if [[ "${released}" != "${version}" ]]; then
  echo "The subject names ${version} and ${manifest} holds '${released}'; not tagging." >&2
  exit 1
fi

# The pull requests a commit on main came from, rebased or not, narrowed to the
# one release-please labeled.
pr_number="$(gh api "repos/{owner}/{repo}/commits/$(git rev-parse HEAD)/pulls" \
  --jq "[.[] | select(.merged_at != null) | select(any(.labels[]; .name == \"${pending_label}\")) | .number] | first // empty")"

if [[ -z "${pr_number}" ]]; then
  echo "No merged pull request labeled '${pending_label}' holds this commit; not tagging." >&2
  exit 1
fi

echo "Tagging $(git rev-parse --short HEAD) as ${tag}, released by pull request #${pr_number}"
git tag "${tag}"
git push origin "${tag}"

echo "Dispatching release.yml for ${tag}"
gh workflow run release.yml --ref "${tag}"

# Flip release-please's own bookkeeping label, so its next run sees this release
# as handled rather than still pending. The release is already on its way, so a
# failure here is said out loud and does not undo it.
echo "Marking PR #${pr_number} as tagged"
gh pr edit "${pr_number}" --remove-label "${pending_label}" --add-label 'autorelease: tagged' \
  || echo "Could not relabel PR #${pr_number}; release-please may offer this release again." >&2
