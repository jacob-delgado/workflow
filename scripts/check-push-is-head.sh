#!/usr/bin/env bash
#
# Refuse a push of a branch at any commit but the one checked out, from
# lefthook's pre-push hook.
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
# branch a pull request is judged by, so neither is refused. Given no lines,
# as a run of the hook by hand is (TRADE-39), it passes: the checks then judge
# HEAD, which is what the next push from here carries.
#
# Exits 0 when every branch pushed would be at HEAD's commit, and 1 when one
# would not.
set -euo pipefail

head="$(git rev-parse --verify --quiet HEAD || true)"
readonly head
status=0

while read -r local_ref local_sha remote_ref _; do
  if [[ "${remote_ref}" != refs/heads/* || "${local_sha}" =~ ^0+$ || "${local_sha}" == "${head}" ]]; then
    continue
  fi
  echo "check-push-is-head: ${remote_ref} would be ${local_ref} (${local_sha:0:8}), not HEAD (${head:0:8}):" \
    "the pre-push checks judge the working tree, so push a branch from a checkout of it" >&2
  status=1
done

exit "${status}"
