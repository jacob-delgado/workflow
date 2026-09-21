# Package & directory size budget history

The per-directory file budgets in `scripts/package-size-budgets.txt` get bumped
when a grouping legitimately grows, and ratcheted down when a split lands. This
is the running record of why each number moved, so a future maintainer doesn't
have to `git blame` a comment block.

Append a row when you change a number. Date in ISO 8601; **PR** is whichever PR
landed the change; **From → To** is the budget column. One line, no
soft-pedaling.

Numbers that go **down** get a row too — a ratchet after a split is the most
valuable row in this table, because it is the only evidence the debt was
actually paid.

The two legitimate reasons for an upward bump: the new file is the same
responsibility spelled one concern wider, or the directory's responsibility
genuinely grew and splitting would separate things that change together. "The
gate was in the way" is not one of them.

| Date | PR | Directory | From → To | Rationale |
| --- | --- | --- | --- | --- |
| 2026-09-21 | #94 | *(all)* | — → initial | Gate introduced. Default is 12; two directories earned registration at their current source-file count, zero headroom. Everything else answers to the default. |
| 2026-09-21 | #94 | `internal/tui` | — → 32 | The Bubble Tea interface, one file per pane/overlay/composer, all hanging behavior off the shared `Model`. Cohesive by construction and unsplittable without threading `Model` across a package boundary; frozen so a new concern is a deliberate bump, not drift. |
| 2026-09-21 | #94 | `internal/webserver` | — → 15 | The loopback REST surface, one handler per operation plus the shared guard and error envelope. Grows only when the OpenAPI spec gains an operation, which is already a recorded decision. |
| 2026-09-21 | #96 | `internal/tui` | 32 → 34 | `issuewrite.go`: the assign and log-work forms on the Issues pane — one overlay, parameterized by the action, for the two one-field Jira writes. `issuekeys.go`: the Issues pane's key routing, split out of `detail.go` when adding the two verbs pushed it past the 500-line file ceiling. Both are the same responsibility (drive the terminal UI) spelled file-per-concern, not a second reason to change. |
| 2026-09-21 | #103 | `internal/tui` | 34 → 35 | `scopesuggest.go`: the commit scope field's completion from the shared staged directory and the scopes already in `git log` (FEAT-21). Kept beside `composer.go` rather than folded into it, which would push `composer.go` toward the 500-line file ceiling; the same responsibility (drive the terminal UI) spelled file-per-concern, not a second reason to change. |
| 2026-09-21 | #105 | `internal/tui` | 35 → 36 | `diff.go`: the Commits pane's diff preview — reading and drawing the selected file's diff below the list, marked by more than color (FEAT-18). Kept beside `commits.go` (already 450 lines) rather than folded in, which would push it past the 500-line file ceiling; the same responsibility (drive the terminal UI) spelled file-per-concern. |
| 2026-09-21 | FEAT-17 | `internal/tui` | 36 → 37 | `finish.go`: the Review pane's finish-a-merged-branch preview and the three-git-command cleanup it runs, plus the merged-pull detail (FEAT-17). Split out of `review.go` when the merge action (FEAT-31) and the finish action together pushed it past the 800-line hard file ceiling; the same responsibility (drive the terminal UI) spelled file-per-concern, not a second reason to change. |
