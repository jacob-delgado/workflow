#!/usr/bin/env bash
#
# Fail if a copy of the mise version differs from mise.toml's min_version.
#
# Usage:
#   scripts/check-mise-version.sh          # the gate
#   scripts/check-mise-version.sh --list   # each source and the version read
#
# mise cannot pin its own binary: it reads mise.toml only once it is installed.
# So wherever mise gets installed the version is stated again — the `version:`
# on every jdx/mise-action step under .github/workflows, and mise_version in
# .devcontainer/postCreate.sh — and min_version is the floor they all install.
# A floor raised alone fails every CI job at its mise-action step; a workflow
# bumped alone runs a mise nobody else does. So every copy must equal the floor,
# and a mise-action step with no version, installing whatever mise is newest,
# fails too.
#
# Paths are read from the current directory, so `task` runs it from the repo
# root and its own test runs it against a fixture tree.
set -euo pipefail

# A bracket expression matching either YAML quote, handed to awk as a variable
# because a single quote cannot sit inside the single-quoted programs below.
readonly quotes="[\"']"

fail() {
  echo "check-mise-version: $*" >&2
  exit 1
}

# assignment prints "<file>:<line><TAB><value>" for the first line of a file
# that matches a pattern, the value being what follows its `=`, unquoted. It
# fails rather than letting an empty match pass as agreement.
assignment() {
  local file="$1" pattern="$2" row
  [[ -f "${file}" ]] || fail "no ${file}"
  row="$(awk -v pattern="${pattern}" -v quotes="${quotes}" '
    $0 ~ pattern {
      value = $0
      sub(/^[^=]*=[ \t]*/, "", value)
      sub(/[ \t]+#.*$/, "", value)
      gsub(quotes, "", value)
      print FILENAME ":" FNR "\t" value
      exit
    }' "${file}")"
  [[ -n "${row}" ]] || fail "could not read the mise version from ${file}"
  echo "${row}"
}

# workflow_steps prints one "<file>:<line><TAB><version>" row per jdx/mise-action
# step in the workflows. A step's keys sit at the column of its `uses:`, so a
# line indented less ends the step, and a `version:` before that is the one the
# step installs. A step that names no version prints an empty value, pointing at
# its `uses:` line.
workflow_steps() {
  local workflows=() workflow
  for workflow in .github/workflows/*.yml .github/workflows/*.yaml; do
    if [[ -f "${workflow}" ]]; then
      workflows+=("${workflow}")
    fi
  done
  ((${#workflows[@]} > 0)) || return 0

  awk -v quotes="${quotes}" '
    function indent(line) {
      match(line, /[^ ]/)
      return RSTART - 1
    }
    function unquote(text) {
      sub(/^[ \t]+/, "", text)
      sub(/[ \t]+#.*$/, "", text)
      sub(/[ \t]+$/, "", text)
      gsub(quotes, "", text)
      return text
    }
    function close_step() {
      if (open) print where "\t" version
      open = 0
    }
    FNR == 1 { close_step() }
    open && $0 !~ /^[ \t]*(#.*)?$/ {
      if (indent($0) < key) {
        close_step()
      } else if ($0 ~ /^[ \t]*version:/) {
        value = $0
        sub(/^[ \t]*version:/, "", value)
        version = unquote(value)
        where = FILENAME ":" FNR
      }
    }
    !open && $0 ~ "^[ \t]*(-[ \t]+)?uses:[ \t]*" quotes "?jdx/mise-action@" {
      open = 1
      version = ""
      where = FILENAME ":" FNR
      key = index($0, "uses:") - 1
    }
    END { close_step() }
  ' "${workflows[@]}"
}

floor_row="$(assignment "mise.toml" '^min_version[ \t]*=')"
devcontainer_row="$(assignment ".devcontainer/postCreate.sh" '^readonly mise_version=')"
steps="$(workflow_steps)"
[[ -n "${steps}" ]] \
  || fail "read no jdx/mise-action step under .github/workflows, so nothing was held to min_version"

readonly floor="${floor_row#*$'\t'}"
readonly copies="${steps}"$'\n'"${devcontainer_row}"

if [[ "${1:-}" == "--list" ]]; then
  while IFS=$'\t' read -r where version; do
    printf '%-40s %s\n' "${where}" "${version:-(none)}"
  done <<<"${floor_row}"$'\n'"${copies}"
  exit 0
fi

if [[ -n "${1:-}" ]]; then
  sed -n '3,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' >&2
  exit 2
fi

drift=""
count=0
while IFS=$'\t' read -r where version; do
  count=$((count + 1))
  if [[ -z "${version}" ]]; then
    drift+="  ${where}: states no version"$'\n'
  elif [[ "${version}" != "${floor}" ]]; then
    drift+="  ${where}: ${version}"$'\n'
  fi
done <<<"${copies}"

if [[ -n "${drift}" ]]; then
  echo "check-mise-version: these copies differ from min_version, ${floor_row%%$'\t'*}: ${floor}" >&2
  printf '%s' "${drift}" >&2
  echo "Install one mise everywhere: move every copy with min_version, and the devcontainer's checksums with its copy." >&2
  exit 1
fi

echo "check-mise-version: all ${count} copies install mise ${floor}, mise.toml's min_version."
