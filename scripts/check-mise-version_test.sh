#!/usr/bin/env bash
#
# Tests for check-mise-version.sh: it passes when every workflow's mise-action
# step and the devcontainer install mise.toml's min_version, and fails on any
# copy that differs, on a mise-action step with no version, on a source it
# cannot read, and when it read no mise-action at all.
#
# Usage:
#   scripts/check-mise-version_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-mise-version.sh"

workdir="$(mktemp -d)"
readonly workdir
trap 'rm -rf "${workdir}"' EXIT

failures=0
cases=0

# run_gate runs the gate in a directory, keeping its output in <dir>.out, and
# prints pass or fail.
run_gate() {
  local dir="$1"
  if (cd "${dir}" && "${check}") >"${dir}.out" 2>&1; then
    echo "pass"
  else
    echo "fail"
  fi
}

# expect runs the gate in a directory and compares its exit to pass or fail.
#   expect <pass|fail> <name> <dir>
expect() {
  local want="$1" name="$2" dir="$3" got
  cases=$((cases + 1))
  got="$(run_gate "${dir}")"

  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL ${name}: want ${want}, got ${got}" >&2
    sed 's/^/  /' "${dir}.out" >&2
    failures=$((failures + 1))
  fi
}

# expect_named runs the gate in a directory, wants it to fail, and wants its
# output to carry the given text.
#   expect_named <name> <dir> <text>
expect_named() {
  local name="$1" dir="$2" text="$3" got
  cases=$((cases + 1))
  got="$(run_gate "${dir}")"

  if [[ "${got}" != "fail" ]] || ! grep -qF -- "${text}" "${dir}.out"; then
    echo "FAIL ${name}: want a failure naming '${text}', got ${got}:" >&2
    sed 's/^/  /' "${dir}.out" >&2
    failures=$((failures + 1))
  fi
}

# tree writes mise.toml, two workflows holding three mise-action steps between
# them, and the devcontainer's install script into a fresh directory. Each
# workflow also sets a `version:` on another action, which is not mise's.
#   tree <dir> <min_version> <ci lint> <ci test> <release> <devcontainer>
tree() {
  local dir="$1" floor="$2" lint="$3" test="$4" release="$5" devcontainer="$6"
  mkdir -p "${dir}/.github/workflows" "${dir}/.devcontainer"
  printf 'min_version = "%s"\n\n[tools]\ngo = "1.27.1"\n' "${floor}" >"${dir}/mise.toml"
  cat >"${dir}/.github/workflows/ci.yml" <<EOF
jobs:
  lint:
    steps:
      - uses: jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c # v4.3.0
        with:
          version: ${lint} # the mise binary itself
      - uses: example/other-action@v1
        with:
          version: 0.0.1
      - run: task lint
  test:
    steps:
      - uses: jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c # v4.3.0
        with:
          cache: false
          version: "${test}"
      - run: task test
EOF
  cat >"${dir}/.github/workflows/release.yml" <<EOF
jobs:
  release:
    steps:
      - name: Provision the toolchain
        if: always()
        uses: jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c # v4.3.0
        with:
          version: '${release}'
          cache: false
  docs:
    steps:
      - uses: example/other-action@v1
        with:
          version: 9.9.9
EOF
  printf '#!/usr/bin/env bash\nset -euo pipefail\n\nreadonly mise_version="%s"\n' \
    "${devcontainer}" >"${dir}/.devcontainer/postCreate.sh"
}

# Every copy installs the floor, whatever the other actions' versions say.
agree="${workdir}/agree"
tree "${agree}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.9.3
expect pass "every copy agrees" "${agree}"

# One workflow's step is edited alone; the gate names the line and the value.
workflow="${workdir}/workflow"
tree "${workflow}" 2026.9.3 2026.9.3 2026.9.3 2026.9.4 2026.9.3
expect_named "one workflow edited alone" "${workflow}" \
  ".github/workflows/release.yml:8: 2026.9.4"

# A version in quotes is still a version, and still has to match.
quoted="${workdir}/quoted"
tree "${quoted}" 2026.9.3 2026.9.3 2026.9.2 2026.9.3 2026.9.3
expect_named "a quoted version that differs" "${quoted}" \
  ".github/workflows/ci.yml:16: 2026.9.2"

# The devcontainer downloads a different mise than the floor.
devcontainer="${workdir}/devcontainer"
tree "${devcontainer}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.8.0
expect_named "the devcontainer differs" "${devcontainer}" \
  ".devcontainer/postCreate.sh:4: 2026.8.0"

# The floor is raised without the copies following it.
floor="${workdir}/floor"
tree "${floor}" 2026.9.4 2026.9.3 2026.9.3 2026.9.3 2026.9.3
expect_named "the floor raised alone" "${floor}" "mise.toml:1: 2026.9.4"

# A mise-action step with no version installs whatever mise is newest.
unpinned="${workdir}/unpinned"
tree "${unpinned}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.9.3
cat >"${unpinned}/.github/workflows/pages.yml" <<'EOF'
jobs:
  pages:
    steps:
      - uses: jdx/mise-action@c2a87611a18de5b3828c5652fe268e992400cb5c # v4.3.0
        with:
          cache: false
      - uses: example/other-action@v1
        with:
          version: 2026.9.3
EOF
expect_named "a mise-action step with no version" "${unpinned}" \
  ".github/workflows/pages.yml:4"

# Workflows that use no mise-action leave nothing to compare: refuse to pass.
none="${workdir}/none"
tree "${none}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.9.3
rm "${none}/.github/workflows/ci.yml" "${none}/.github/workflows/release.yml"
printf 'jobs:\n  lint:\n    steps:\n      - run: task lint\n' >"${none}/.github/workflows/ci.yml"
expect fail "no mise-action read" "${none}"

# A source the gate cannot read must stop it, not pass on what it did read.
missing="${workdir}/missing"
tree "${missing}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.9.3
rm "${missing}/.devcontainer/postCreate.sh"
expect fail "a source that cannot be read" "${missing}"

# A mise.toml with no floor leaves nothing to hold the copies to.
nofloor="${workdir}/nofloor"
tree "${nofloor}" 2026.9.3 2026.9.3 2026.9.3 2026.9.3 2026.9.3
printf '[tools]\ngo = "1.27.1"\n' >"${nofloor}/mise.toml"
expect fail "a mise.toml with no min_version" "${nofloor}"

if ((failures > 0)); then
  echo "check-mise-version_test: ${failures} of ${cases} case(s) failed." >&2
  exit 1
fi

echo "check-mise-version_test: ${cases} case(s) passed."
