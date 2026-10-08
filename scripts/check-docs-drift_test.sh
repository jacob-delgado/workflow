#!/usr/bin/env bash
#
# Tests for check-docs-drift.sh: it passes when the generator reproduces the
# committed reference and fails when the two disagree — the drift that would
# otherwise ship documentation describing a binary that no longer exists.
#
# The real generator is the command tree; a stand-in stands in for it here, so
# the test settles the compare-and-fail logic without regenerating the whole
# reference.
#
# Usage:
#   scripts/check-docs-drift_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly check="${here}/check-docs-drift.sh"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

# The committed reference: a hand-written _index.md and one generated page.
reference="${workdir}/reference"
mkdir -p "${reference}"
printf 'the index\n' >"${reference}/_index.md"
printf 'the command\n' >"${reference}/workflow.md"

# in_sync writes the same generated page the reference holds; drift writes a
# different one. Each takes the output directory as its one argument.
in_sync="${workdir}/in-sync.sh"
cat >"${in_sync}" <<'EOF'
#!/usr/bin/env bash
printf 'the command\n' >"$1/workflow.md"
EOF
chmod +x "${in_sync}"

drift="${workdir}/drift.sh"
cat >"${drift}" <<'EOF'
#!/usr/bin/env bash
printf 'a different command\n' >"$1/workflow.md"
EOF
chmod +x "${drift}"

# expect runs the gate with a generator and checks its exit.
#   expect <pass|fail> <name> <generator>
expect() {
  expect_exit "$1" "$2" env DOCS_REFERENCE="${reference}" DOCSGEN="$3" "${check}"
}

# A generator that reproduces the committed reference passes.
expect pass "the reference is current" "${in_sync}"

# A generator whose output differs fails — the check's whole point.
expect fail "the reference has drifted" "${drift}"

finish_tests
