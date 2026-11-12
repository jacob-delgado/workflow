#!/usr/bin/env bash
#
# Tests for lefthook.yml's pre-commit hook: the fixers, which rewrite the
# staged files and stage them again, never run beside a check that reads those
# files, nor beside another fixer of the same files, so no check reads a file
# half written and no two fixers write one at once; and every fixer and check
# a staged Go, shell, Markdown and TOML file reaches still runs. Stand-ins for
# each tool note when they start and stop, and run long enough that any two
# lefthook starts together are seen together, so nothing is formatted or
# checked for real. It runs the lefthook mise.toml pins, and is skipped where
# none is on PATH.
#
# Usage:
#   scripts/lefthook_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

command -v lefthook >/dev/null || {
  echo "lefthook_test.sh: skipped, no lefthook here"
  exit 0
}

readonly stub_bin="${workdir}/bin" marks="${workdir}/marks" repo="${workdir}/repo"
mkdir -p "${stub_bin}" "${marks}/active"

# stub writes a stand-in for a tool that notes it ran, as a fixer or a check
# of a kind of file, and notes each other tool running beside it that it must
# not: a check beside a fixer, or a fixer beside one of the same files. It
# marks itself running first and looks for others after, so of two that start
# together each sees the other.
#   stub <path> <name, or "golangci-lint" for its fmt and run> <fixer|check> <files>
stub() {
  cat >"$1" <<STUB
#!/usr/bin/env bash
name="$2"
if [[ "\${name}" == golangci-lint ]]; then name="golangci-lint \$1"; fi
me="${marks}/active/$3.$4.\${name// /-}"
printf '%s\n' "\${name}" >>"${marks}/ran"
: >"\${me}"
sleep 0.5
for other in "${marks}"/active/*; do
  [[ -e "\${other}" && "\${other}" != "\${me}" ]] || continue
  read -r kind files <<<"\$(basename "\${other}" | tr . ' ')"
  if [[ "\${kind} $3" == *fixer* && ( "\${kind} $3" != "fixer fixer" || "\${files%% *}" == "$4" ) ]]; then
    printf '%s beside %s\n' "\${name}" "\$(basename "\${other}")" >>"${marks}/overlaps"
  fi
done
sleep 0.5
rm -f "\${me}"
STUB
  chmod +x "$1"
}

# The repository: lefthook.yml, a stand-in for the license-header check it
# runs from scripts/, and a first commit, so golangci-lint has a HEAD to
# compare with; then a Go, a shell, a Markdown and a TOML file staged.
git init -q "${repo}"
mkdir -p "${repo}/scripts"
cp "${here}/../lefthook.yml" "${repo}/lefthook.yml"
stub "${repo}/scripts/check-license-headers.sh" license-headers check go
git -C "${repo}" add -A
git -C "${repo}" -c user.email=t@example.com -c user.name=t commit -q -m first
stub "${stub_bin}/golangci-lint" golangci-lint fixer go
stub "${stub_bin}/shfmt" shfmt fixer sh
stub "${stub_bin}/markdownlint-cli2" markdownlint-cli2 fixer md
stub "${stub_bin}/taplo" taplo fixer toml
stub "${stub_bin}/shellcheck" shellcheck check sh
stub "${stub_bin}/typos" typos check all
stub "${stub_bin}/gitleaks" gitleaks check all
printf 'package a\n' >"${repo}/a.go"
printf '#!/usr/bin/env bash\n' >"${repo}/b.sh"
printf '# B\n' >"${repo}/c.md"
printf 'a = 1\n' >"${repo}/d.toml"
git -C "${repo}" add a.go b.sh c.md d.toml

# pre_commit runs lefthook's pre-commit in the repository as git's commit
# would, with the stand-ins first on PATH, whatever a hook variable the test
# runs under would skip.
pre_commit() {
  run_in "${repo}" env -u LEFTHOOK -u LEFTHOOK_EXCLUDE PATH="${stub_bin}:${PATH}" \
    lefthook run pre-commit --no-auto-install
}

expect_exit pass "the pre-commit hook" pre_commit
count_case
if [[ -s "${marks}/overlaps" ]]; then
  fail_case "the pre-commit hook" "a fixer ran beside what reads or writes its files: $(sort -u "${marks}/overlaps")"
fi
for tool in "golangci-lint fmt" "golangci-lint run" license-headers shfmt shellcheck \
  markdownlint-cli2 taplo typos gitleaks; do
  count_case
  if ! grep -qx "${tool}" "${marks}/ran"; then
    fail_case "the pre-commit hook" "${tool} did not run: $(tr '\n' ',' <"${marks}/ran")"
  fi
done

finish_tests
