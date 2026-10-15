#!/usr/bin/env bash
#
# Tests for check-tech-debt-paths.sh, how the Markdown around a path is read:
# it fails on a path in the indented paragraphs of a list item, bulleted with
# -, * or + (after a space or a tab), or one going on past a fence inside it,
# ordered with 1. or 1), or itself indented, and names the line a span opens
# on, in a paragraph of several. It reads a paragraph whole and pairs backtick
# runs of a length within it, so a span after a longer one on its line and a
# span that wraps onto the next line are read, and a paragraph ends at a blank
# line, a heading, a new list item, a fence, a setext underline or a thematic
# break; it reads a run of backticks nothing closes, and a line opening with a
# triple-backtick span, as text; it skips code fenced with ``` or ~~~, indented
# up to three spaces or, in a list item, more, though a line of tildes indented
# four spaces in a paragraph is no fence (a fence closing only on a run of its
# own character at least as long, with nothing but spaces or tabs after it),
# indented code blocks, by spaces or a tab, wherever one can open (after a
# fence, a heading, a bare one too, a setext underline of = or -, indented or
# not, or a thematic break, at the top of the file, after a list has ended,
# after a line of spaces) but not where a list item or a paragraph goes on, and
# spans of two or more backticks, a wrapped one too; and it reads a span
# holding a backtick or wrapped with a space as no path.
# check-tech-debt-paths_test.sh tests what reads as a repository path and how
# one is looked up.
#
# Usage:
#   scripts/check-tech-debt-paths_markdown_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"
# shellcheck source=lib/tech-debt-paths-fixtures.sh
source "${here}/lib/tech-debt-paths-fixtures.sh"

expect_backlog fail "a path after a code span wrapped onto the next line" "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
A command, `go test
./internal/tui`, then `internal/tui/spine.go`.
EOF

expect_backlog pass "fenced code, skipped" "1 repository path(s)" <<'EOF'
```sh
echo `internal/gone.go`
```

~~~sh
echo `internal/gone.go`
~~~

Then `scripts/gate.sh`.
EOF

expect_backlog pass "a fence holding a shorter fence, skipped whole" "1 repository path(s)" <<'EOF'
````md
```sh
echo `internal/gone.go`
```
echo `internal/gone.go`
````

Then `scripts/gate.sh`.
EOF

expect_backlog pass "a fence that a fence of the other kind does not close" "1 repository path(s)" <<'EOF'
```sh
echo `internal/gone.go`
~~~
echo `internal/gone.go`
```

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block, skipped" "1 repository path(s)" <<'EOF'
Text.

    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog fail "a list item's indented paragraph, read" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
- An item.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "a line opening with a triple-backtick span, no fence" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
```internal/tui/model.go``` is it.

Then `internal/tui/spine.go`.
EOF

expect_backlog pass "a fence that a fence line with an info string does not close" "1 repository path(s)" <<'EOF'
```
```sh
echo `internal/gone.go`
```

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block on the first line, skipped" "1 repository path(s)" <<'EOF'
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block right after a fence, skipped" "1 repository path(s)" <<'EOF'
A paragraph runs into a fence.
```
x
```
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block right after a heading, skipped" "1 repository path(s)" <<'EOF'
# Technical debt
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a list has ended, skipped" "1 repository path(s)" <<'EOF'
- An item.

Text.

    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a list a fence ended, skipped" "1 repository path(s)" <<'EOF'
- An item.

```
x
```

    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a list a heading ended, skipped" "1 repository path(s)" <<'EOF'
- An item.
# Heading

    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog fail "a path after a triple-backtick span on its line" "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
```a``` and `internal/tui/spine.go` here.
EOF

expect_backlog fail "a path after a fence that follows a lone backtick" "TECH_DEBT.md:5: internal/tui/spine.go" <<'EOF'
A lone ` here
```
x
```
Then `internal/tui/spine.go`.
EOF

expect_backlog fail "a list item's second indented paragraph, read" "TECH_DEBT.md:5: internal/tui/spine.go" <<'EOF'
- An item.

    Its second paragraph.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "a list item going on past a line starting with #" "TECH_DEBT.md:4: internal/tui/spine.go" <<'EOF'
- An item
#7 is an issue, no heading.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "a paragraph going on after seven hashes" "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
####### is no heading
    and `internal/tui/spine.go` goes on its paragraph.
EOF

expect_backlog pass "an indented code block right after a setext heading, skipped" "1 repository path(s)" <<'EOF'
Technical debt
==============
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a thematic break ends a list, skipped" "1 repository path(s)" <<'EOF'
- An item.

* * *

    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "a double-backtick span, not read" "1 repository path(s)" <<'EOF'
``internal/gone.go`` is quoted; see `scripts/gate.sh`.
EOF

expect_backlog pass "a span holding a longer run of backticks, no path" "1 repository path(s)" <<'EOF'
`internal/tui/``model.go` holds backticks; see `scripts/gate.sh`.
EOF

expect_backlog pass "a span wrapped mid-path, read with its space as no path" "1 repository path(s)" <<'EOF'
A span `internal/tui/
gone.go` wraps with a space; see `scripts/gate.sh`.
EOF

expect_backlog fail "a path after a heading's unclosed backtick" "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
# Debt `x
Then `internal/tui/spine.go`.
EOF

expect_backlog fail "a path in a heading after a paragraph's unclosed backtick" "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
Text `x
# Heading on `internal/tui/spine.go`
EOF

expect_backlog fail "a path in a list item after one with an unclosed backtick" "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
- a `x
- b `internal/tui/spine.go`
EOF

expect_backlog fail "a path after a run of backticks nothing closes" "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
Use `` to quote; see `internal/tui/spine.go`.
EOF

expect_backlog fail "a path in the paragraph after a lone backtick" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
A lone ` here.

Then `internal/tui/spine.go`.
EOF

expect_backlog fail "a path after a setext heading's unclosed backtick" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
Debt `x
=======
Then `internal/tui/spine.go`.
EOF

expect_backlog fail "a path after a thematic break that ends an unclosed backtick" \
  "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
Text `x
***
Then `internal/tui/spine.go`.
EOF

expect_backlog fail "a path on the first line of a longer paragraph, named by its line" \
  "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
It lives in `internal/tui/spine.go`,
as the next line says.
EOF

expect_backlog fail "a path opening a later line of a paragraph, named by its line" \
  "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
The first line, and
`internal/tui/spine.go` opens the second.
EOF

expect_backlog fail "a path opening a line after a span, named by its line" \
  "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
See `a` and
`internal/tui/spine.go` here.
EOF

expect_backlog fail "a path opening a line after a double span holding a backtick, named by its line" \
  "TECH_DEBT.md:2: internal/tui/spine.go" <<'EOF'
See `` a ` b `` and
`internal/tui/spine.go` here.
EOF

expect_backlog fail "a padded span opening at a line's end after two spans, named by its line" \
  "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
See `a`, `b` and `
internal/tui/spine.go ` here.
EOF

expect_backlog fail "a padded span opening at a line's end after a double span holding two backticks" \
  "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
See `` a ` b ` c `` and `
internal/tui/spine.go ` here.
EOF

expect_backlog fail "a * list item's paragraph, read" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
* An item.

    It lives in `internal/tui/spine.go`.
EOF

# shellcheck disable=SC2016 # the backticks are Markdown, not a command
expect_backlog fail "a list item after a tab, its paragraph read" "TECH_DEBT.md:3: internal/tui/spine.go" \
  < <(printf -- '-\tAn item.\n\n    It lives in `internal/tui/spine.go`.\n')

expect_backlog pass "an indented code block right after a thematic break, skipped" "1 repository path(s)" <<'EOF'
Text.
***
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog fail "a line of tildes indented four spaces in a paragraph, no fence" \
  "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
Text.
    ~~~
Then `internal/tui/spine.go`.
EOF

expect_backlog pass "a fence inside a list item, indented past three spaces, skipped" "1 repository path(s)" <<'EOF'
1. Step:

    ~~~
    ls `internal/gone.go`
    ~~~

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a setext underline of dashes, skipped" "1 repository path(s)" <<'EOF'
Title
--
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after a bare heading, skipped" "1 repository path(s)" <<'EOF'
#
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog pass "a fence indented by up to three spaces, skipped" "1 repository path(s)" <<'EOF'
   ~~~
echo `internal/gone.go`
   ~~~

Then `scripts/gate.sh`.
EOF

expect_backlog fail "a list going on past a fence inside an item" "TECH_DEBT.md:7: internal/tui/spine.go" <<'EOF'
- An item.

  ```
  x
  ```

    It lives in `internal/tui/spine.go`.
EOF

# shellcheck disable=SC2016 # the backticks are Markdown, not a command
expect_backlog pass "an indented code block after a line of spaces, skipped" "1 repository path(s)" \
  < <(printf 'Text.\n   \n    echo `internal/gone.go`\n\nThen `scripts/gate.sh`.\n')

expect_backlog pass "a fence closed by a longer run, skipped" "1 repository path(s)" <<'EOF'
```
echo `internal/gone.go`
````

Then `scripts/gate.sh`.
EOF

expect_backlog pass "an indented code block after an indented setext underline, skipped" "1 repository path(s)" <<'EOF'
Title
  ===
    echo `internal/gone.go`

Then `scripts/gate.sh`.
EOF

expect_backlog fail "a path after runs of two and three backticks, neither closed" \
  "TECH_DEBT.md:1: internal/tui/spine.go" <<'EOF'
Use `` with ``` here; see `internal/tui/spine.go`.
EOF

# shellcheck disable=SC2016 # the backticks are Markdown, not a command
expect_backlog fail "a path after a fence closed by a run and a tab" "TECH_DEBT.md:7: internal/tui/spine.go" \
  < <(printf '# Technical debt\n\n```\nx\n```\t\n\nThen `internal/tui/spine.go`.\n')

# shellcheck disable=SC2016 # the backticks are Markdown, not a command
expect_backlog fail "a path after a fence closed by a run and spaces" "TECH_DEBT.md:7: internal/tui/spine.go" \
  < <(printf '# Technical debt\n\n```\nx\n```  \n\nThen `internal/tui/spine.go`.\n')

expect_backlog fail "a + list item's paragraph, read" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
+ An item.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "a 1) list item's paragraph, read" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
1) An item.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog pass "a double-backtick span wrapped onto the next line, not read" "1 repository path(s)" <<'EOF'
Quote ``a `internal/gone.go`
b`` here; see `scripts/gate.sh`.
EOF

expect_backlog fail "an indented list item's paragraph, read" "TECH_DEBT.md:5: internal/tui/spine.go" <<'EOF'
Text.

  - An indented item.

    It lives in `internal/tui/spine.go`.
EOF

expect_backlog fail "an ordered list item's paragraph, read" "TECH_DEBT.md:3: internal/tui/spine.go" <<'EOF'
1. An item.

    It lives in `internal/tui/spine.go`.
EOF

# shellcheck disable=SC2016 # the backticks are Markdown, not a command
expect_backlog pass "a code block indented with a tab, skipped" "1 repository path(s)" \
  < <(printf '# Technical debt\n\nText.\n\n\techo `internal/gone.go`\n\nThen `scripts/gate.sh`.\n')

finish_tests
