# shellcheck shell=bash
#
# Reading what each file the age gate dates pins, one pin a line: go.mod's
# requirements and replacements, web/yarn.lock's resolutions, and mise.toml's
# tools. Each reads the file on stdin, so the gate reads it as
# it stands at HEAD and at the merge base, and dates each line that is new.
#
# Sourced, never run, after the script's own `set -euo pipefail`:
#
#   # shellcheck source=lib/dependency-pins.sh
#   source "${here}/lib/dependency-pins.sh"

# go_module_pins prints "module version" for each requirement in a go.mod,
# and for each module version a replace directive puts in a module's place,
# which the build takes instead of the one required. A replacement by a
# directory names no version and comes from no proxy, so it prints nothing.
go_module_pins() {
  awk '
    function replacement(line,   words) {
      sub(/[ \t]*\/\/.*$/, "", line)
      if (sub(/^.*=>[ \t]*/, "", line) && split(line, words) >= 2) print words[1], words[2]
    }
    /^require[ \t]*\(/ { block = "require"; next }
    /^replace[ \t]*\(/ { block = "replace"; next }
    block != "" && /^\)/ { block = ""; next }
    block == "require" && NF >= 2 && $1 !~ /^\/\// { print $1, $2; next }
    block == "replace" { replacement($0); next }
    /^require[ \t]+[^ \t(]/ { print $2, $3 }
    /^replace[ \t]+[^ \t(]/ { replacement($0) }
  '
}

# yarn_resolutions prints "package version" for each npm package a yarn.lock
# resolves.
yarn_resolutions() {
  sed -n -E 's/^  resolution: "(.+)@npm:([0-9][^"]*)"$/\1 \2/p'
}

# mise_pins prints "tool version" for each [tools] pin in a mise.toml, and for
# its min_version as the tool "mise". A pin is a version string or an inline
# table holding one, as `vhs = { version = "0.12.1", os = ["linux"] }`. A
# table's header is read as TOML reads it, whatever spaces, quotes or comment
# it is written with. An inline table naming no version, a [tools.<name>]
# table, and a tools key before any table, dotted or an inline table, which
# mise reads as pins too, print "! line" instead: this reads no version from
# any of them, so the gate fails it rather than let a version through undated.
mise_pins() {
  awk '
    function value(line) { sub(/^[^=]*=[ \t]*"/, "", line); sub(/".*/, "", line); return line }
    function inline_version(line) {
      if (!match(line, /[{,][ \t]*version[ \t]*=[ \t]*"[^"]*"/)) return ""
      return value(substr(line, RSTART + 1, RLENGTH - 1))
    }
    function pin(line,   key, version) {
      key = line; sub(/[ \t]*=.*/, "", key); gsub(/"/, "", key)
      if (line !~ /^[^=]*=[ \t]*\{/) { print key, value(line); return }
      version = inline_version(line)
      if (version == "") print "!", line
      else print key, version
    }
    function header(line) { sub(/#.*$/, "", line); gsub(/[ \t"\047]/, "", line); return line }
    /^min_version[ \t]*=/ { print "mise", value($0); next }
    /^[ \t]*\[/ {
      in_table = 1
      tools = (header($0) == "[tools]")
      if (header($0) ~ /^\[tools\./) print "!", $0
      next
    }
    !in_table && /^[ \t]*["\047]?tools["\047]?[ \t]*[.=]/ { print "!", $0; next }
    tools && /^[^#].*=/ { pin($0) }
  '
}
