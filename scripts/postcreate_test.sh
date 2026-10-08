#!/usr/bin/env bash
#
# Tests for .devcontainer/postCreate.sh's last step: it installs the git hooks
# in any checkout git can read — a clone, or a `git worktree add` checkout,
# whose .git is a file rather than a directory — and outside a repository says
# to run `task setup` once there is one. A stand-in mise records what it was
# asked and does nothing, so no toolchain is installed.
#
# Usage:
#   scripts/postcreate_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly postcreate="${here}/../.devcontainer/postCreate.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly stub_bin="${workdir}/bin"
mkdir -p "${stub_bin}"
cat >"${stub_bin}/mise" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${MISE_STUB_LOG}"
STUB
chmod +x "${stub_bin}/mise"

# workspace copies the script into a fresh workspace, with the mise.toml it
# trusts, and prints its path.
#   workspace <dir>
workspace() {
  mkdir -p "$1/.devcontainer"
  cp "${postcreate}" "$1/.devcontainer/postCreate.sh"
  printf 'min_version = "2026.9.3"\n' >"$1/mise.toml"
}

# commit makes a repository of a workspace, with its files committed.
#   commit <dir>
commit() {
  git -C "$1" init -q
  git -C "$1" add -A
  git -C "$1" -c user.email=t@example.com -c user.name=t commit -q -m workspace
}

# provision runs a workspace's postCreate.sh with the stand-in mise and a home
# of its own.
#   provision <dir>
provision() {
  mkdir -p "$1.home"
  env PATH="${stub_bin}:${PATH}" HOME="$1.home" MISE_STUB_LOG="$1.mise" \
    "$1/.devcontainer/postCreate.sh"
}

# What the stand-in mise is asked when postCreate.sh installs the hooks.
readonly install_hooks='exec -- lefthook install'

clone="${workdir}/clone"
workspace "${clone}"
commit "${clone}"
expect_output pass "a clone gets its hooks" "lefthook install" provision "${clone}"
count_case
if ! grep -qx "${install_hooks}" "${clone}.mise"; then
  fail_case "a clone's hooks" "mise was not asked to install them"
fi

main="${workdir}/main"
workspace "${main}"
commit "${main}"
git -C "${main}" worktree add -q "${workdir}/linked"
expect_output pass "a linked worktree gets its hooks" "lefthook install" provision "${workdir}/linked"
count_case
if ! grep -qx "${install_hooks}" "${workdir}/linked.mise"; then
  fail_case "a linked worktree's hooks" "mise was not asked to install them"
fi

bare="${workdir}/bare"
workspace "${bare}"
expect_output pass "no repository yet" "run 'task setup' after 'git init'" provision "${bare}"
count_case
if grep -qx "${install_hooks}" "${bare}.mise"; then
  fail_case "no repository's hooks" "mise was asked to install hooks with no repository"
fi

finish_tests
