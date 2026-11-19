#!/usr/bin/env bash
#
# Refuse a push of a branch at any commit but the one checked out, or from a
# working tree with changes, from lefthook's pre-push hook.
#
# Usage:
#   scripts/check-push-is-head.sh < <the lines git hands a pre-push hook>
#
# The pre-push hook's checks run over the working tree, and lefthook.yml picks
# them by what HEAD changes against origin/main, so they judge the commit
# checked out. A push of any other, as `git push origin feat/x` from main,
# would be judged by HEAD's tests and lint instead of its own, and by none at
# all where HEAD changes nothing. git hands the hook a line for each ref it
# pushes, "<local ref> <local sha> <remote ref> <remote sha>"; this fails,
# naming the ref, when a branch it pushes would not be at HEAD's commit. A
# deletion, whose local sha is all zeros, pushes no commit, and a tag is no
# branch a pull request is judged by, so neither is refused. The checks read
# the working tree too, so a push of a branch also fails, naming the first
# entries, when `git status --porcelain` finds anything: a change to a tracked
# file, staged or not, or a file git does not ignore, which `go test` would
# compile though the push carries none of it, listed even where git is set to
# hide untracked files from its status. Given no lines, as a run of the
# hook by hand is, git handing it nothing, it passes and looks at nothing:
# the checks then judge the working tree, which is what the next commit and
# push carry.
#
# lefthook.yml runs this as pre-push's first job, and its refusal stops the
# hook before any other check.
#
# Exits 0 when every branch pushed would be at HEAD's commit from a working
# tree with no changes, and 1 when one would not.
set -euo pipefail

head="$(git rev-parse --verify --quiet HEAD || true)"
readonly head
status=0
pushes_branch=""

while read -r local_ref local_sha remote_ref _; do
  if [[ "${remote_ref}" != refs/heads/* || "${local_sha}" =~ ^0+$ ]]; then
    continue
  fi
  pushes_branch=yes
  [[ "${local_sha}" == "${head}" ]] && continue
  echo "check-push-is-head: ${remote_ref} would be ${local_ref} (${local_sha:0:8}), not HEAD (${head:0:8}):" \
    "the pre-push checks judge the working tree, so push a branch from a checkout of it" >&2
  status=1
done

if [[ -n "${pushes_branch}" ]]; then
  changes="$(git status --porcelain --untracked-files=normal)"
  if [[ -n "${changes}" ]]; then
    echo "check-push-is-head: the working tree has changes the push does not carry:" >&2
    sed -n '1,5s/^/  /p' <<<"${changes}" >&2
    echo "check-push-is-head: the pre-push checks judge the working tree; commit or stash first" >&2
    status=1
  fi
fi

exit "${status}"
