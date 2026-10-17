# shellcheck shell=bash
#
# Reading what each file the age gate dates pins, one pin a line: go.mod's
# requirements, web/yarn.lock's resolutions, the files
# build/yamllint/requirements.txt pins by their hashes and the lines it pins
# what cannot be dated with, build/markdownlint/package-lock.json's entries,
# and mise.toml's tools. Each reads the file on stdin, so the gate reads it as
# it stands at HEAD and at the merge base, and dates each line that is new.
#
# Sourced, never run, after the script's own `set -euo pipefail`:
#
#   # shellcheck source=lib/dependency-pins.sh
#   source "${here}/lib/dependency-pins.sh"

# go_requires prints "module version" for each requirement in a go.mod.
go_requires() {
  awk '
    /^require[ \t]*\(/ { block = 1; next }
    block && /^\)/ { block = 0; next }
    block && NF >= 2 && $1 !~ /^\/\// { print $1, $2; next }
    /^require[ \t]+[^ \t(]/ { print $2, $3 }
  '
}

# yarn_resolutions prints "package version" for each npm package a yarn.lock
# resolves.
yarn_resolutions() {
  sed -n -E 's/^  resolution: "(.+)@npm:([0-9][^"]*)"$/\1 \2/p'
}

# requirements_pins prints "package version sha256" for each file a pip
# requirements file pins by its hash, "package version" for a requirement
# pinned by none, and "! line" for a line that pins what cannot be dated. A
# requirement runs on over each line that ends in a backslash, and a comment
# ends a line, as pip reads them; a line that starts with a comment ends one
# that runs on and never runs on itself, whatever it ends with. A line holding
# a character outside tab and printable ASCII is refused, since pip also ends
# a line at a form feed and at the other breaks Python's splitlines knows,
# where this reads one. The gate dates only what
# scripts/pin-yamllint.sh writes: a comment or a blank line, its two options,
# and name==version requirements whose every word after the version is
# --hash=sha256:<64 hex digits>. Any other line, an include, an index, a
# direct URL, an editable install, a marker, or a hash of another kind or
# spelling among them, could have pip install a file no hash here names, so it
# is refused rather than skipped.
requirements_pins() {
  LC_ALL=C awk '
    function refuse(line) { print "!", line }
    function pin(line,   name, version, words, count, i) {
      sub(/(^|[ \t]+)#.*$/, "", line)
      gsub(/[ \t]+/, " ", line); sub(/^ /, "", line); sub(/ $/, "", line)
      if (line == "" || line == "--require-hashes" || line == "--only-binary :all:") return
      if (!match(line, /^[A-Za-z0-9][A-Za-z0-9._-]* ?(\[[A-Za-z0-9._, -]*\])? ?== ?[A-Za-z0-9][^ ;\\]*/)) {
        refuse(line)
        return
      }
      name = line; sub(/[^A-Za-z0-9._-].*/, "", name)
      version = substr(line, 1, RLENGTH); sub(/^[^=]*== ?/, "", version)
      count = split(substr(line, RLENGTH + 1), words)
      for (i = 1; i <= count; i++) {
        if (words[i] !~ /^--hash=sha256:[0-9a-f]+$/ || length(words[i]) != 78) { refuse(line); return }
      }
      for (i = 1; i <= count; i++) print name, version, substr(words[i], 15)
      if (!count) print name, version
    }
    /[^\t -~]/ { if (joined != "") pin(joined); refuse($0); joined = ""; next }
    /^[ \t]*#/ { if (joined != "") pin(joined); joined = ""; next }
    { joined = joined $0 }
    joined ~ /\\$/ { sub(/\\$/, " ", joined); next }
    { pin(joined); joined = "" }
    END { if (joined != "") pin(joined) }
  '
}

# npm_lock_pins prints "package version resolved integrity" for each package
# an npm package-lock.json installs under a node_modules: an aliased one by
# the name in its entry, any other by its install path's last name, and "-"
# for a version, a resolved or an integrity the entry lacks. Where it comes
# from and its hash are on the line, so an entry changed in either alone is
# new to the gate too. The lockfile's root sits under no node_modules, and a
# package linked from a directory, marked link, comes from no registry, so
# neither is printed. A lock npm would read other than by its packages map, a
# lockfileVersion 1 by its dependencies, and one jq cannot parse print one
# "! why" line instead, since npm ci installs what the gate would not see; an
# absent lock, read as nothing, prints nothing.
npm_lock_pins() {
  jq -r 'if type != "object" then "! it is not a JSON object"
    elif (.lockfileVersion | IN(2, 3) | not) then
      "! its lockfileVersion is \(.lockfileVersion // "missing"), not the 2 or 3 whose packages map npm installs from"
    elif (.packages | type) != "object" then "! it has no packages map"
    else .packages | to_entries[]
      | select((.key | test("(^|/)node_modules/")) and .value.link != true)
      | "\(.value.name // (.key | sub("^.*node_modules/"; ""))) \(.value.version // "-") \(.value.resolved // "-") \(.value.integrity // "-")"
    end' 2>/dev/null || echo "! it is not valid JSON"
}

# mise_pins prints "tool version" for each [tools] pin in a mise.toml, and for
# its min_version as the tool "mise".
mise_pins() {
  awk '
    function value(line) { sub(/^[^=]*=[ \t]*"/, "", line); sub(/".*/, "", line); return line }
    /^min_version[ \t]*=/ { print "mise", value($0); next }
    /^\[/ { tools = ($0 == "[tools]"); next }
    tools && /^[^#].*=/ { key = $0; sub(/[ \t]*=.*/, "", key); gsub(/"/, "", key); print key, value($0) }
  '
}
