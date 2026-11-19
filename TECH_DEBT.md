# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan,
kept for a contributor looking for something worth fixing and for a later
Claude Code session asked to pick up an entry by its ID.
[FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas; this file holds
the debts.

A choice made on purpose is not debt. It is explained by a comment at its
code, where a reader or a reviewer meets it, not here. A security problem
goes in neither: report it as [SECURITY.md](SECURITY.md) asks.

## How to write an entry

An entry is a `### DEBT-n <title>` section under *Open*, with the next unused
ID, DEBT-331 onward: an ID is never reused. One line under the title rates
it, as `Severity: low · Confidence: read · Size: S`, ending in `· Breaking`
when its fix changes a contract a user or a script depends on. Four
paragraphs follow: **Where.** names the functions and files rather than
lines, which drift; **Today.** says what is wrong and what it costs;
**Fix.** gives one way to fix it; and **Done when.** says how to tell it is
fixed, observably, so a test or a command can assert it.

A fixed debt is deleted in the commit that fixes it; git keeps its history.

## Open

None.
