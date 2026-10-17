#!/usr/bin/env bash
#
# Tests for pin-yamllint.sh: it prints a requirements file that pins yamllint at
# the version asked for and every package it needs at run time, those its
# requirements need too and none an extra alone needs, each once, at its newest
# final release that is not yanked and whose every file is a week old, unless a
# <package>==<version> names another, its name matched as PyPI spells it; each
# by the sha256 of every wheel its release holds, sorted, and of no sdist, under
# the options that hold pip to them. A package PyPI does not know, one with no
# release old enough and a release with no wheel stop it, by name, and so does a
# requirement with an environment marker, which it does not pin, and a
# <package>==<version> naming yamllint or a package the tree does not hold; a
# package named twice, in any spelling PyPI reads as one, is refused with 2
# before any call; a run that stops prints no part of the file, and a call it
# cannot read is refused with a status of its own. A release exactly a week old
# is old enough, and one a second younger is not.
#
# A stand-in curl answers each PyPI URL from a fixture file named for it, and
# fails as curl -f does for any other, so no case reaches the network.
# PIN_YAMLLINT_NOW fixes "now" at 2026-10-08T00:00:00Z.
#
# Usage:
#   scripts/pin-yamllint_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly pin="${here}/pin-yamllint.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

readonly now=1791417600 # 2026-10-08T00:00:00Z
readonly old="2026-09-20T10:00:00Z" young="2026-10-06T10:00:00.123Z"

readonly stub_bin="${workdir}/bin" fixtures="${workdir}/fixtures"
mkdir -p "${stub_bin}" "${fixtures}"
cat >"${stub_bin}/curl" <<'STUB'
#!/usr/bin/env bash
url="${*: -1}"
fixture="${CURL_STUB_FIXTURES}/$(printf '%s' "${url}" | tr -c 'A-Za-z0-9.' '_')"
if [[ -f "${fixture}" ]]; then
  cat "${fixture}"
else
  exit 22
fi
STUB
chmod +x "${stub_bin}/curl"

# answer records what the stand-in curl returns for a URL.
#   answer <url> <body>
answer() {
  printf '%s\n' "$2" >"${fixtures}/$(printf '%s' "$1" | tr -c 'A-Za-z0-9.' '_')"
}

# hash_of prints a sha256 of one repeated hex digit.
#   hash_of <digit>
hash_of() {
  printf "$1%.0s" {1..64}
}

# file prints a PyPI file: its kind, its sha256, when it was uploaded and,
# given "yanked", that it is yanked.
#   file <bdist_wheel|sdist> <sha256> <time> [yanked]
file() {
  jq -cn --arg kind "$1" --arg sha256 "$2" --arg time "$3" --arg yanked "${4:-}" \
    '{packagetype: $kind, digests: {sha256: $sha256}, upload_time_iso_8601: $time, yanked: ($yanked == "yanked")}'
}

# release answers a release's JSON: what it requires, then its files.
#   release <package> <version> <requires-dist-json> [file-json...]
release() {
  local package="$1" version="$2" requires="$3"
  shift 3
  answer "https://pypi.org/pypi/${package}/${version}/json" \
    "$(jq -cn --argjson requires "${requires}" '{info: {requires_dist: $requires}, urls: $ARGS.positional}' \
      --jsonargs "$@")"
}

# index answers a package's JSON, whose releases map each version to its files.
#   index <package> <releases-json>
index() {
  answer "https://pypi.org/pypi/$1/json" "{\"releases\": $2}"
}

aged="$(file bdist_wheel "$(hash_of 0)" "${old}")"
fresh="$(file bdist_wheel "$(hash_of 0)" "${young}")"
aged_yanked="$(file bdist_wheel "$(hash_of 0)" "${old}" yanked)"
readonly aged fresh aged_yanked

# yamllint 1.38.0 needs pathspec and PyYAML, and its dev extra needs doc8.
release yamllint 1.38.0 '["pathspec>=1.0.0", "PyYAML", "doc8; extra == \"dev\""]' \
  "$(file bdist_wheel "$(hash_of a)" "${old}")" "$(file sdist "$(hash_of 5)" "${old}")"
# pathspec's newest release is two days old, the one before it yanked, and a
# release candidate is no final one; 1.1.10 is newer than 1.1.9.
index pathspec "{\"1.1.9\": [${aged}], \"1.1.10\": [${aged}], \"1.1.11\": [${aged_yanked}],
  \"1.2.0\": [${fresh}], \"1.3.0rc1\": [${aged}]}"
release pathspec 1.1.10 'null' "$(file bdist_wheel "$(hash_of b)" "${old}")"
# pyyaml 6.0.3 took a wheel two days ago, so 6.0.2 is its newest whose every
# file is a week old. It needs Helper_Lib, which needs pathspec again.
index pyyaml "{\"6.0.2\": [${aged}, ${aged}], \"6.0.3\": [${aged}, ${fresh}]}"
release pyyaml 6.0.2 '["Helper_Lib"]' \
  "$(file bdist_wheel "$(hash_of d)" "${old}")" "$(file bdist_wheel "$(hash_of c)" "${old}")" \
  "$(file sdist "$(hash_of 6)" "${old}")"
release pyyaml 6.0.3 '["Helper_Lib"]' "$(file bdist_wheel "$(hash_of e)" "${young}")"
index helper-lib "{\"2.0.0\": [${aged}]}"
release helper-lib 2.0.0 '["pathspec"]' "$(file bdist_wheel "$(hash_of f)" "${old}")"

# yamllint 9.9.1 has an sdist alone, 9.9.2 needs a package PyPI does not know,
# and 9.9.3 one with no release a week old.
release yamllint 9.9.1 'null' "$(file sdist "$(hash_of 5)" "${old}")"
release yamllint 9.9.2 '["nowhere"]' "$(file bdist_wheel "$(hash_of a)" "${old}")"
release yamllint 9.9.3 '["fresh"]' "$(file bdist_wheel "$(hash_of a)" "${old}")"
index fresh "{\"1.0.0\": [${fresh}]}"

# yamllint 9.9.4 needs edge, whose 1.1.0 is a week old to the second, now
# being 2026-10-08T00:00:00Z, and whose 1.2.0 is a second short of it.
release yamllint 9.9.4 '["edge"]' "$(file bdist_wheel "$(hash_of a)" "${old}")"
index edge "{\"1.0.0\": [${aged}],
  \"1.1.0\": [$(file bdist_wheel "$(hash_of 1)" 2026-10-01T00:00:00Z)],
  \"1.2.0\": [$(file bdist_wheel "$(hash_of 2)" 2026-10-01T00:00:01Z)]}"
release edge 1.1.0 'null' "$(file bdist_wheel "$(hash_of 7)" "${old}")"

# yamllint 9.9.5 needs pathspec everywhere, and tomli on an older Python alone.
release yamllint 9.9.5 '["pathspec", "tomli; python_version < \"3.11\""]' \
  "$(file bdist_wheel "$(hash_of a)" "${old}")"

# pinned runs the script with the stand-in curl.
#   pinned [arg...]
pinned() {
  env PATH="${stub_bin}:${PATH}" CURL_STUB_FIXTURES="${fixtures}" PIN_YAMLLINT_NOW="${now}" "${pin}" "$@"
}

# pinned_on_stdout runs pinned with its errors left out, so a case reads only
# what it printed of the file.
#   pinned_on_stdout [arg...]
pinned_on_stdout() {
  pinned "$@" 2>/dev/null
}

expect_output pass "the options that hold pip to the hashes and to wheels" \
  "--require-hashes
--only-binary :all:
yamllint==1.38.0" pinned 1.38.0

expect_output pass "yamllint at the version asked for, by its wheel and not its sdist" \
  "yamllint==1.38.0 \\
    --hash=sha256:$(hash_of a)
pathspec==" pinned 1.38.0

expect_output pass "a dependency at its newest final release a week old, not yanked" \
  "pathspec==1.1.10 \\
    --hash=sha256:$(hash_of b)
pyyaml==" pinned 1.38.0

expect_output pass "a dependency's newest release whose every file is a week old, by every wheel, sorted" \
  "pyyaml==6.0.2 \\
    --hash=sha256:$(hash_of c) \\
    --hash=sha256:$(hash_of d)
helper-lib==2.0.0 \\
    --hash=sha256:$(hash_of f)" pinned 1.38.0

expect_output pass "a version named for a dependency, its name matched as PyPI spells it, taken at any age" \
  "pyyaml==6.0.3 \\
    --hash=sha256:$(hash_of e)" pinned 1.38.0 PyYAML==6.0.3

expect_output 1 "a version named for a package not in the tree" \
  "pin-yamllint: pyyml is named, but yamllint 1.38.0 does not need it" pinned 1.38.0 pyyml==6.0.3
expect_exit 1 "a version named for a package not in the tree" pinned_on_stdout 1.38.0 pyyml==6.0.3
count_case
if [[ -s "${case_output}" ]]; then
  fail_case "a version named for a package not in the tree, printing no part of the file" \
    "it printed: $(cat "${case_output}")"
fi

expect_output 1 "a version named for yamllint itself" \
  "pin-yamllint: yamllint==1.37.0 is named, but yamllint's version is the first argument, 1.38.0" \
  pinned 1.38.0 yamllint==1.37.0

left_out="doc8|$(hash_of 5)|$(hash_of 6)"
expect_exit pass "the tree yamllint 1.38.0 needs" pinned 1.38.0
count_case
if grep -qE "${left_out}" "${case_output}"; then
  fail_case "an extra's dependency and an sdist left out" "$(cat "${case_output}")"
fi
count_case
if [[ "$(grep -c '^pathspec==' "${case_output}")" != 1 ]]; then
  fail_case "a package two others need, pinned once" "$(cat "${case_output}")"
fi

expect_output pass "a dependency's release a week old to the second, and none younger" \
  "edge==1.1.0 \\
    --hash=sha256:$(hash_of 7)" pinned 9.9.4

expect_output 1 "a requirement with an environment marker" \
  "pin-yamllint: yamllint 9.9.5 requires 'tomli; python_version < \"3.11\"', and a marker is not pinned: pin it by hand or teach this script markers" \
  pinned 9.9.5
expect_exit 1 "a run that stops" pinned_on_stdout 9.9.5
count_case
if [[ -s "${case_output}" ]]; then
  fail_case "a run that stops, printing no part of the file" "it printed: $(cat "${case_output}")"
fi

expect_output 1 "a release with no wheel" "yamllint 9.9.1 has no wheel" pinned 9.9.1
expect_output 1 "a package PyPI does not know" "cannot read https://pypi.org/pypi/nowhere/json" pinned 9.9.2
expect_output 1 "a package with no release a week old" "fresh has no final release a week old" pinned 9.9.3
expect_exit 2 "no yamllint version" pinned
expect_exit 2 "a dependency named with no version" pinned 1.38.0 pyyaml
expect_output 2 "a dependency named twice, as PyPI spells it" "pyyaml is named twice" \
  pinned 1.38.0 PyYAML==6.0.3 pyyaml==6.0.2
expect_exit 2 "a dependency named twice prints nothing" pinned_on_stdout 1.38.0 PyYAML==6.0.3 pyyaml==6.0.2
count_case
if [[ -s "${case_output}" ]]; then
  fail_case "a dependency named twice prints nothing" "it printed: $(cat "${case_output}")"
fi

finish_tests
