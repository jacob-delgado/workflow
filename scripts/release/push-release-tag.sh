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
# Environment:
#   HEAD_COMMIT_MSG  the pushed commit's message, passed as an env var rather
#                    than interpolated into the workflow's run: line, because its
#                    content is attacker-influenceable.
#   GH_TOKEN         needs actions:write (workflow run) and pull-requests:write
#                    (label flip).
set -euo pipefail

readonly head_commit_msg="${HEAD_COMMIT_MSG:?HEAD_COMMIT_MSG is required}"

subject="$(printf '%s\n' "${head_commit_msg}" | head -n1)"

# release-please's merge commit subject, e.g. "chore(main): release 1.4.0".
version="$(printf '%s\n' "${subject}" | sed -n -E 's/^chore\(main\): release ([0-9]+\.[0-9]+\.[0-9]+.*)$/\1/p')"

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

echo "Tagging $(git rev-parse --short HEAD) as ${tag}"
git tag "${tag}"
git push origin "${tag}"

echo "Dispatching release.yml for ${tag}"
gh workflow run release.yml --ref "${tag}"

# Flip release-please's own bookkeeping label, so its next run sees this release
# as handled rather than still pending.
pr_number="$(gh pr list --state merged --label 'autorelease: pending' \
  --search "release ${version}" --json number --jq '.[0].number // empty')"

if [[ -n "${pr_number}" ]]; then
  echo "Marking PR #${pr_number} as tagged"
  gh pr edit "${pr_number}" \
    --remove-label 'autorelease: pending' \
    --add-label 'autorelease: tagged' || true
fi
