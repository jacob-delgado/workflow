#!/usr/bin/env bash
#
# Tests for check-dependency-age.sh's hold on the linters' pin files: a file a
# change adds to build/yamllint/requirements.txt by its hash, or a package it
# adds to build/markdownlint/package-lock.json, passes once it was published at
# least seven days before, and fails, by name, when it is younger, unless the
# exceptions file lists it, a lockfile package then needing no web/.yarnrc.yml
# line. A PyPI file is dated on its own, though its release be older, with one
# look-up per release, a requirement whose backslash runs on to the end of the
# file among them, and one after a comment, which never runs on and ends a run;
# a hash PyPI does not list fails, as a requirement pinned by no hash does,
# excepted or not, and so does a line pinning what cannot be dated: another
# hash, its spelling or its length, a direct URL, an include by either option,
# another --only-binary, a character pip would read as a line break. An npm
# lockfile entry is dated as the package it installs, an aliased one too, with
# one look-up per package, and fails, excepted or not, when it names no version,
# does not come from the registry under its own name and version, or has no
# sha512 the registry lists for it, an entry changed in no more than its hash
# among them. A lockfile's own root and its linked packages are not looked up,
# and a lock npm would read other than by its packages map, a lockfileVersion 1
# or one with no such map, fails by its file's name, as one that is not a JSON
# object, or not JSON at all, does. Pins a change leaves alone in either file
# are not looked up.
#
# The stand-in curl lib/check-dependency-age-fixtures.sh writes answers each
# registry URL from a fixture file named for it, and fails as curl -f does for
# any other, so no case reaches the network.
#
# Usage:
#   scripts/check-dependency-age_pins_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/check-dependency-age-fixtures.sh
source "${here}/lib/check-dependency-age-fixtures.sh"

# pyyaml 6.0.4 holds two files a week old and one two days old, as a release
# takes wheels for days after its first; PyPI lists no file hashed unlisted.
hash_of() {
  printf "$1%.0s" {1..64}
}
hash_old="$(hash_of a)" hash_old_too="$(hash_of d)" hash_young="$(hash_of b)" hash_unlisted="$(hash_of c)"
readonly hash_old hash_old_too hash_young hash_unlisted
answer "https://pypi.org/pypi/pyyaml/6.0.4/json" "{\"urls\":[
  {\"digests\":{\"sha256\":\"${hash_old}\"},\"upload_time_iso_8601\":\"${old}\"},
  {\"digests\":{\"sha256\":\"${hash_old_too}\"},\"upload_time_iso_8601\":\"${old}\"},
  {\"digests\":{\"sha256\":\"${hash_young}\"},\"upload_time_iso_8601\":\"${young}\"}]}"
# argparse 2.0.0 and 3.0.2 are both two days old.
answer "https://registry.npmjs.org/argparse" "{\"time\":{\"2.0.0\":\"${young}\",\"3.0.2\":\"${young}\"},
  \"versions\":{\"2.0.0\":{\"dist\":{\"integrity\":\"$(integrity_of argparse 2.0.0)\"}},
  \"3.0.2\":{\"dist\":{\"integrity\":\"$(integrity_of argparse 3.0.2)\"}}}}"

# require_pip adds a requirement to build/yamllint/requirements.txt, pinned by
# the sha256 of each file named, or by none, under the header a generated one
# opens with.
#   require_pip <package> <version> [sha256...]
require_pip() {
  local package="$1" version="$2" sha256
  shift 2
  mkdir -p build/yamllint
  if [[ ! -f build/yamllint/requirements.txt ]]; then
    printf '# Generated.\n--require-hashes\n--only-binary :all:\n' >build/yamllint/requirements.txt
  fi
  {
    printf '%s==%s' "${package}" "${version}"
    for sha256 in "$@"; do
      printf ' \\\n    --hash=sha256:%s' "${sha256}"
    done
    printf '\n'
  } >>build/yamllint/requirements.txt
}

# registry_entry prints the lock entry npm writes for a package version it
# takes from the registry: the tarball's URL there, and the integrity the
# registry lists.
#   registry_entry <package> <version>
registry_entry() {
  jq -cn --arg version "$2" --arg integrity "$(integrity_of "$1" "$2")" \
    --arg resolved "https://registry.npmjs.org/$1/-/${1#@*/}-$2.tgz" \
    '{version: $version, resolved: $resolved, integrity: $integrity}'
}

# lock_entry writes build/markdownlint/package-lock.json installing one entry
# at the path under node_modules npm gives it, beside the lockfile's own root.
#   lock_entry <path> <entry-json>
lock_entry() {
  mkdir -p build/markdownlint
  jq -n --arg key "node_modules/$1" --argjson entry "$2" \
    '{lockfileVersion: 3, packages: {"": {name: "pins", version: "9.9.9"}, ($key): $entry}}' \
    >build/markdownlint/package-lock.json
}

# lock_npm writes build/markdownlint/package-lock.json installing one package
# from the registry at the path under node_modules npm gives it, a nested one
# under another's.
#   lock_npm <path> <version>
lock_npm() {
  lock_entry "$1" "$(registry_entry "${1##*node_modules/}" "$2")"
}

# lock_unhashed locks the old @scope/widget with no integrity.
lock_unhashed() {
  lock_entry @scope/widget "$(registry_entry @scope/widget 2.0.0 | jq -c 'del(.integrity)')"
}

# lock_sha1 locks the old @scope/widget by a sha1 integrity alone.
lock_sha1() {
  lock_entry @scope/widget "$(registry_entry @scope/widget 2.0.0 | jq -c '.integrity = "sha1-AAAA"')"
}

# lock_rehashed locks the old @scope/widget by an integrity the registry does
# not list.
lock_rehashed() {
  lock_entry @scope/widget "$(registry_entry @scope/widget 2.0.0 | jq -c '.integrity = "sha512-Zm9yZ2Vk"')"
}

# lock_foreign locks the old @scope/widget from a tarball on another host.
lock_foreign() {
  lock_entry @scope/widget "$(registry_entry @scope/widget 2.0.0 \
    | jq -c '.resolved = "https://evil.example/widget-2.0.0.tgz"')"
}

# lock_foreign_waived locks the young argparse from a tarball on another host
# and lists it as waived.
lock_foreign_waived() {
  lock_entry argparse "$(registry_entry argparse 3.0.2 | jq -c '.resolved = "https://evil.example/argparse-3.0.2.tgz"')"
  except argparse 3.0.2 waived-2026-10-07
}

# lock_old_widget locks the old @scope/widget as npm does.
lock_old_widget() {
  lock_npm @scope/widget 2.0.0
}

# lock_aliased locks the young argparse at @scope/widget's path, as npm does
# for "@scope/widget": "npm:argparse@2.0.0".
lock_aliased() {
  lock_entry @scope/widget "$(registry_entry argparse 2.0.0 | jq -c '.name = "argparse"')"
}

# lock_aliased_elsewhere locks an alias of argparse whose tarball is
# @scope/widget's.
lock_aliased_elsewhere() {
  lock_entry @scope/widget "$(registry_entry @scope/widget 2.0.0 | jq -c '.name = "argparse"')"
}

# lock_versionless locks the young argparse by its tarball and integrity
# alone, with no version, as a hand edit could leave it.
lock_versionless() {
  lock_entry argparse "$(registry_entry argparse 3.0.2 | jq -c 'del(.version)')"
}

# lock_rehashed_with_advisory locks the old @scope/widget by an integrity the
# registry does not list and lists its version as a fix.
lock_rehashed_with_advisory() {
  lock_rehashed
  except @scope/widget 2.0.0 CVE-2026-12345
}

# lock_shaped writes build/markdownlint/package-lock.json as the text given.
#   lock_shaped <text>
lock_shaped() {
  mkdir -p build/markdownlint
  printf '%s\n' "$1" >build/markdownlint/package-lock.json
}

# lock_v1 locks the young argparse as a lockfileVersion 1 lock does, under
# dependencies with no packages map, which npm ci still installs from.
lock_v1() {
  lock_shaped "$(jq -cn --argjson entry "$(registry_entry argparse 3.0.2)" \
    '{lockfileVersion: 1, dependencies: {argparse: $entry}}')"
}

# lock_without_packages locks the young argparse in a lockfileVersion 2 lock's
# dependencies, with no packages map beside them.
lock_without_packages() {
  lock_shaped "$(jq -cn --argjson entry "$(registry_entry argparse 3.0.2)" \
    '{lockfileVersion: 2, dependencies: {argparse: $entry}}')"
}

# link_npm writes build/markdownlint/package-lock.json holding a package linked
# from a directory beside it, and nothing from the registry.
link_npm() {
  mkdir -p build/markdownlint
  jq -n '{lockfileVersion: 3, packages: {"": {name: "pins", version: "9.9.9"},
    "node_modules/local": {resolved: "local", link: true}, local: {name: "local", version: "1.0.0"}}}' \
    >build/markdownlint/package-lock.json
}

# young_pip_with_advisory requires pyyaml's young file and lists its release
# as a fix.
young_pip_with_advisory() {
  require_pip pyyaml 6.0.4 "${hash_young}"
  except pyyaml 6.0.4 CVE-2026-12345
}

# unhashed_pip_with_advisory requires pyyaml by no hash and lists its release
# as a fix.
unhashed_pip_with_advisory() {
  require_pip pyyaml 6.0.4
  except pyyaml 6.0.4 CVE-2026-12345
}

# unlisted_pip_with_advisory requires pyyaml by a hash PyPI lists no file of
# and lists its release as a fix.
unlisted_pip_with_advisory() {
  require_pip pyyaml 6.0.4 "${hash_unlisted}"
  except pyyaml 6.0.4 CVE-2026-12345
}

# short_hash_pip_with_advisory requires pyyaml by a sha256 one hex digit
# short and lists its release as a fix.
short_hash_pip_with_advisory() {
  write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_old:1}"
  except pyyaml 6.0.4 CVE-2026-12345
}

# long_hash_pip_with_advisory requires pyyaml by a sha256 one hex digit long
# and lists its release as a fix.
long_hash_pip_with_advisory() {
  write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_old}a"
  except pyyaml 6.0.4 CVE-2026-12345
}

# write_requirements writes build/yamllint/requirements.txt as the lines
# given, under the header a generated one opens with.
#   write_requirements <line...>
write_requirements() {
  mkdir -p build/yamllint
  printf '# Generated.\n--require-hashes\n--only-binary :all:\n' >build/yamllint/requirements.txt
  printf '%s\n' "$@" >>build/yamllint/requirements.txt
}

# young_lock_waived locks the young argparse and lists it as waived, with no
# web/.yarnrc.yml line, which only web/yarn.lock needs.
young_lock_waived() {
  lock_npm markdown-it/node_modules/argparse 3.0.2
  except argparse 3.0.2 waived-2026-10-07
}

pip_old="$(repo pip-old require_pip pyyaml 6.0.4 "${hash_old}" "${hash_old_too}")"
expect_output pass "two files a week old of a PyPI release whose newest is younger" \
  "ok    pyyaml 6.0.4 (sha256:${hash_old_too}): published 2026-09-20" age "${pip_old}"
count_case
if [[ "$(grep -c 'pypi.org/pypi/pyyaml/6.0.4/json' "${pip_old}.curl")" != 1 ]]; then
  fail_case "a PyPI release asked of once for all its files" "it looked up: $(cat "${pip_old}.curl")"
fi

pip_young="$(repo pip-young require_pip pyyaml 6.0.4 "${hash_old}" "${hash_young}")"
expect_output fail "a PyPI file two days old, in a release a week old" \
  "pyyaml 6.0.4 (sha256:${hash_young}) was published 1 day(s) ago" age "${pip_young}"

pip_unlisted="$(repo pip-unlisted require_pip pyyaml 6.0.4 "${hash_unlisted}")"
unlisted="is pinned by a hash PyPI lists no file of, or PyPI could not be asked, so it cannot be dated"
expect_output fail "a hash PyPI lists no file of" \
  "pyyaml 6.0.4 (sha256:${hash_unlisted}) ${unlisted}" age "${pip_unlisted}"

pip_unlisted_fix="$(repo pip-unlisted-fix unlisted_pip_with_advisory)"
expect_output fail "a hash PyPI lists no file of, listed with its advisory" \
  "pyyaml 6.0.4 (sha256:${hash_unlisted}) ${unlisted}" age "${pip_unlisted_fix}"

pip_unhashed="$(repo pip-unhashed require_pip pyyaml 6.0.4)"
expect_output fail "a PyPI requirement pinned by no hash" "pyyaml 6.0.4 is pinned by no hash" age "${pip_unhashed}"

pip_unhashed_fix="$(repo pip-unhashed-fix unhashed_pip_with_advisory)"
expect_output fail "a PyPI requirement pinned by no hash, listed with its advisory" \
  "pyyaml 6.0.4 is pinned by no hash" age "${pip_unhashed_fix}"

undatable="cannot be dated: pin by name==version and --hash=sha256 only"
pip_sha512="$(repo pip-sha512 write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_old} --hash=sha512:$(hash_of b)$(hash_of b)")"
expect_output fail "a PyPI requirement hashed by sha512 beside sha256" "${undatable}" age "${pip_sha512}"

pip_spaced="$(repo pip-spaced write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_old} --hash sha256:${hash_young}")"
expect_output fail "a PyPI requirement's hash spelled with a space" "${undatable}" age "${pip_spaced}"

pip_url="$(repo pip-url write_requirements "pyyaml @ https://example.invalid/pyyaml.whl --hash=sha256:${hash_young}")"
expect_output fail "a PyPI requirement pinned by a direct URL" "${undatable}" age "${pip_url}"

pip_short_hash="$(repo pip-short-hash short_hash_pip_with_advisory)"
expect_output fail "a PyPI requirement's sha256 a digit short, listed with its advisory" \
  "'pyyaml==6.0.4 --hash=sha256:${hash_old:1}' ${undatable}" age "${pip_short_hash}"

pip_long_hash="$(repo pip-long-hash long_hash_pip_with_advisory)"
expect_output fail "a PyPI requirement's sha256 a digit long, listed with its advisory" \
  "'pyyaml==6.0.4 --hash=sha256:${hash_old}a' ${undatable}" age "${pip_long_hash}"

pip_included="$(repo pip-included write_requirements "-r extra.txt")"
expect_output fail "a requirements file that includes another" "'-r extra.txt' ${undatable}" age "${pip_included}"

pip_long_include="$(repo pip-long-include write_requirements "--requirement extra.txt")"
expect_output fail "a requirements file that includes another by the long option" \
  "'--requirement extra.txt' ${undatable}" age "${pip_long_include}"

pip_other_binary="$(repo pip-other-binary write_requirements "--only-binary :none:")"
expect_output fail "an --only-binary other than :all:" "'--only-binary :none:' ${undatable}" age "${pip_other_binary}"

# pip never runs a comment on to the next line, whatever it ends with, so the
# requirement after one is its own.
pip_comment_runs_on="$(repo pip-comment-runs-on write_requirements "# pinned by hand \\" \
  "pyyaml==6.0.4 --hash=sha256:${hash_young}")"
expect_output fail "a requirement after a comment that ends in a backslash" \
  "pyyaml 6.0.4 (sha256:${hash_young}) was published 1 day(s) ago" age "${pip_comment_runs_on}"

pip_comment_ends="$(repo pip-comment-ends write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_old} \\" \
  "# a note \\" "pyyaml==6.0.4 --hash=sha256:${hash_young}")"
expect_output fail "a requirement after a comment that ends another's run" \
  "pyyaml 6.0.4 (sha256:${hash_young}) was published 1 day(s) ago" age "${pip_comment_ends}"

# pip also ends a line at a form feed and at the other breaks Python's
# splitlines knows, so a requirement can hide in what reads as one comment.
pip_form_feed="$(repo pip-form-feed write_requirements "$(printf '# a note\fpyyaml==6.0.4 --hash=sha256:%s' "${hash_young}")")"
expect_output fail "a comment holding a form feed" "${undatable}" age "${pip_form_feed}"

pip_trailing="$(repo pip-trailing write_requirements "pyyaml==6.0.4 --hash=sha256:${hash_young} \\")"
expect_output fail "a requirement whose backslash runs on to the end of the file" \
  "pyyaml 6.0.4 (sha256:${hash_young}) was published 1 day(s) ago" age "${pip_trailing}"

pip_fix="$(repo pip-fix young_pip_with_advisory)"
expect_output pass "a young PyPI file listed with its advisory" "pyyaml 6.0.4 (sha256:${hash_young}): fixes CVE-2026-12345" \
  age "${pip_fix}"

lock_young="$(repo lock-young lock_npm markdown-it/node_modules/argparse 3.0.2)"
expect_output fail "an npm lockfile package nested under another, two days old" \
  "argparse 3.0.2 was published 1 day(s) ago" age "${lock_young}"

lock_old="$(repo lock-old lock_npm @scope/widget 2.0.0)"
expect_output pass "a scoped npm lockfile package a week old, beside the lockfile's root" \
  "ok    @scope/widget 2.0.0: published 2026-09-20" age "${lock_old}"

lock_linked="$(repo lock-linked link_npm)"
expect_exit pass "a lockfile's root and a linked package" age "${lock_linked}"
count_case
if [[ -s "${lock_linked}.curl" ]]; then
  fail_case "a lockfile's root and a linked package" "it looked up: $(cat "${lock_linked}.curl")"
fi

lock_waived="$(repo lock-waived young_lock_waived)"
expect_output pass "a young npm lockfile package waived with no web/.yarnrc.yml line" \
  "argparse 3.0.2: waived on 2026-10-07 by the maintainer" age "${lock_waived}"
count_case
if grep -q "^warn" "${case_output}"; then
  fail_case "a young npm lockfile package waived with no web/.yarnrc.yml line" "it was warned of: $(cat "${case_output}")"
fi

count_case
if [[ "$(grep -c 'registry.npmjs.org/@scope%2Fwidget' "${lock_old}.curl")" != 1 ]]; then
  fail_case "an npm package's registry document asked of once" "it looked up: $(cat "${lock_old}.curl")"
fi

lock_unhashed="$(repo lock-unhashed lock_unhashed)"
expect_output fail "an npm lockfile package pinned by no integrity" \
  "@scope/widget 2.0.0 is pinned by no sha512 integrity" age "${lock_unhashed}"

lock_sha1="$(repo lock-sha1 lock_sha1)"
expect_output fail "an npm lockfile package pinned by a sha1 alone" \
  "@scope/widget 2.0.0 is pinned by no sha512 integrity" age "${lock_sha1}"

lock_rehashed="$(repo lock-rehashed lock_rehashed)"
expect_output fail "an npm lockfile package by a hash the registry does not list" \
  "@scope/widget 2.0.0 is locked by a hash the npm registry does not list" age "${lock_rehashed}"

lock_rehashed_fix="$(repo lock-rehashed-fix lock_rehashed_with_advisory)"
expect_output fail "an npm lockfile entry re-hashed at its version, listed with its advisory" \
  "@scope/widget 2.0.0 is locked by a hash the npm registry does not list" age "${lock_rehashed_fix}"

# young_pins pins the young PyPI file and the young lockfile package.
young_pins() {
  require_pip pyyaml 6.0.4 "${hash_young}"
  lock_npm markdown-it/node_modules/argparse 3.0.2
}

pins_kept="$(repo_on pins-kept young_pins touch unrelated.txt)"
expect_exit pass "young pins the change leaves alone" age "${pins_kept}"
count_case
if grep -Eq 'pypi.org|registry.npmjs.org/argparse' "${pins_kept}.curl"; then
  fail_case "young pins the change leaves alone" "they were looked up: $(cat "${pins_kept}.curl")"
fi

relocked_hash="$(repo_on relocked-hash lock_old_widget lock_rehashed)"
expect_output fail "an npm lockfile entry re-hashed at the version it had" \
  "@scope/widget 2.0.0 is locked by a hash the npm registry does not list" age "${relocked_hash}"

lock_foreign="$(repo lock-foreign lock_foreign)"
expect_output fail "an npm lockfile package from another host" \
  "@scope/widget 2.0.0 does not come from the npm registry under its own name and version" age "${lock_foreign}"

relocked_foreign="$(repo_on relocked-foreign lock_old_widget lock_foreign)"
expect_output fail "an npm lockfile entry moved to another host at the version it had" \
  "@scope/widget 2.0.0 does not come from the npm registry under its own name and version" age "${relocked_foreign}"

lock_foreign_waived="$(repo lock-foreign-waived lock_foreign_waived)"
expect_output fail "a waived npm lockfile package from another host" \
  "argparse 3.0.2 does not come from the npm registry under its own name and version" age "${lock_foreign_waived}"

lock_aliased="$(repo lock-aliased lock_aliased)"
expect_output fail "an aliased npm lockfile package, dated as the package it installs" \
  "argparse 2.0.0 was published 1 day(s) ago" age "${lock_aliased}"
count_case
if grep -q 'registry.npmjs.org/@scope%2Fwidget' "${lock_aliased}.curl"; then
  fail_case "an aliased npm lockfile package, dated as the package it installs" \
    "it looked up its path's name: $(cat "${lock_aliased}.curl")"
fi

lock_aliased_elsewhere="$(repo lock-aliased-elsewhere lock_aliased_elsewhere)"
expect_output fail "an aliased npm lockfile package from another package's tarball" \
  "argparse 2.0.0 does not come from the npm registry under its own name and version" age "${lock_aliased_elsewhere}"

lock_versionless="$(repo lock-versionless lock_versionless)"
expect_output fail "an npm lockfile entry with no version" "argparse - names no version" age "${lock_versionless}"

relock="cannot be dated: re-lock it with task deps:markdownlint"
lock_v1="$(repo lock-v1 lock_v1)"
expect_output fail "a lockfileVersion 1 lock, with no packages map" \
  "build/markdownlint/package-lock.json: its lockfileVersion is 1, not the 2 or 3 whose packages map npm installs from, so it ${relock}" \
  age "${lock_v1}"

lock_without_packages="$(repo lock-without-packages lock_without_packages)"
expect_output fail "a lockfileVersion 2 lock with no packages map" \
  "build/markdownlint/package-lock.json: it has no packages map, so it ${relock}" age "${lock_without_packages}"

lock_not_json="$(repo lock-not-json lock_shaped '{"lockfileVersion": 3, "packages": {')"
expect_output fail "a lock that is not JSON" \
  "build/markdownlint/package-lock.json: it is not valid JSON, so it ${relock}" age "${lock_not_json}"

lock_not_object="$(repo lock-not-object lock_shaped '[]')"
expect_output fail "a lock that is no JSON object" \
  "build/markdownlint/package-lock.json: it is not a JSON object, so it ${relock}" age "${lock_not_object}"

finish_tests
