#!/usr/bin/env bash
# Serve `workflow --web` from a throwaway repository, for the server-backed
# Playwright run (web/playwright.server.config.ts), which starts it.
#
# Usage: e2e-server.sh <fixture-dir> <port>
#
# The fixture is rebuilt under <fixture-dir> on every run:
#
#   home/        the server's home directory, empty but for what it writes
#   origin.git   a bare repository standing in for the remote
#   repo/        the repository served: main, one commit, pushed to origin;
#                docs/notes, made from it with no upstream and checked out,
#                since a spec publishes a branch of its own and never the
#                base; and one untracked file, notes.txt, for a spec to stage
#
# The server runs from repo/ with no configuration file and without --dry-run,
# so a write the page sends lands in git: nothing else is there to reach. It
# serves on 127.0.0.1:<port>; the Playwright run passes a port beside the
# default, not the default itself, so the run exercises --port and never meets
# a developer's own `workflow --web` or the Vite preview's /api proxy, which
# both use the default.
#
# It serves the binary `task build` makes, bin/workflow with the web app
# embedded, and does not build it: the spec is about that binary.
set -euo pipefail

readonly fixture_dir="${1:?usage: e2e-server.sh <fixture-dir> <port>}"
readonly port="${2:?usage: e2e-server.sh <fixture-dir> <port>}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root
readonly binary="${repo_root}/bin/workflow"
readonly home="${fixture_dir}/home"
readonly origin="${fixture_dir}/origin.git"
readonly repo="${fixture_dir}/repo"

if [[ ! -x "${binary}" ]]; then
  echo "e2e-server: no ${binary}; run \`task build\` first" >&2
  exit 1
fi

# hermetic prefixes a command so it runs with the fixture's home and the
# caller's PATH and nothing else from the caller's environment: no git
# configuration, token, state directory or GIT_DIR of the developer's, or of a
# hook that ran this, reaches the fixture or the server.
readonly -a hermetic=(env -i "PATH=${PATH}" "HOME=${home}" GIT_CONFIG_NOSYSTEM=1)

# The path is cleared only when it is missing, an empty directory or one this
# script made, which it marks, so a mistyped <fixture-dir> - a file, a symbolic
# link or a directory of someone else's - is refused rather than removed.
readonly marker="${fixture_dir}/.workflow-e2e-fixture"
if [[ -L "${fixture_dir}" || (-e "${fixture_dir}" && ! -d "${fixture_dir}") ]]; then
  echo "e2e-server: ${fixture_dir} is not a directory; not clearing it" >&2
  exit 1
fi
if [[ -d "${fixture_dir}" && ! -f "${marker}" ]] \
  && [[ -n "$(find "${fixture_dir}" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  echo "e2e-server: ${fixture_dir} holds files this script did not make; not clearing it" >&2
  exit 1
fi
rm -rf -- "${fixture_dir}"
mkdir -p -- "${home}" "${repo}"
touch -- "${marker}"

"${hermetic[@]}" git init --quiet --bare --initial-branch=main -- "${origin}"
"${hermetic[@]}" git init --quiet --initial-branch=main -- "${repo}"
"${hermetic[@]}" git -C "${repo}" config user.name 'Workflow E2E'
"${hermetic[@]}" git -C "${repo}" config user.email 'e2e@example.com'
printf '# Fixture\n' >"${repo}/README.md"
"${hermetic[@]}" git -C "${repo}" add README.md
"${hermetic[@]}" git -C "${repo}" commit --quiet --message 'chore: start the fixture'
"${hermetic[@]}" git -C "${repo}" remote add origin "${origin}"
"${hermetic[@]}" git -C "${repo}" push --quiet --set-upstream origin main
"${hermetic[@]}" git -C "${repo}" switch --quiet --create docs/notes
printf 'Staged, committed and pushed through the page.\n' >"${repo}/notes.txt"

cd "${repo}"
exec "${hermetic[@]}" "${binary}" --web --port "${port}"
