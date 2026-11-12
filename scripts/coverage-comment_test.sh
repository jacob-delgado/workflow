#!/usr/bin/env bash
#
# Tests for the script ci.yml's Coverage comment job runs to post the CI
# report on a pull request: it updates the comment the workflow's own bot
# posted with the report's marker, and posts a new one when there is none,
# leaving alone a comment anybody else wrote, though it quotes the marker; the
# report names the head it measured in a marker of its own. A docs-only run,
# which measures nothing, edits that comment to a note of one line naming
# both its own head and the head last measured, reading no report, and posts
# nothing when there is no comment yet. The job is let run for a docs-only
# change. The script is read out of ci.yml as the job runs it and run under
# node with a stand-in for the GitHub client that lists the comments a case
# gives and notes what it was asked to update or create, and with what. It is
# skipped where no node is on PATH.
#
# Usage:
#   scripts/coverage-comment_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly workflow="${here}/../.github/workflows/ci.yml"

# shellcheck source=lib/testing.sh
source "${here}/lib/testing.sh"

command -v node >/dev/null || {
  echo "coverage-comment_test.sh: skipped, no node here"
  exit 0
}

# The upsert step's script, every line under its `script: |` indented past
# the key, with that indentation taken off.
readonly script="${workdir}/upsert.js"
awk '
  /^ *- name: Upsert the comment$/ { step = 1; next }
  step && !indent && /^ *script: \|$/ { match($0, /^ */); indent = RLENGTH; next }
  indent && /[^ ]/ { match($0, /^ */); if (RLENGTH <= indent) exit }
  indent { print substr($0, indent + 3) }
' "${workflow}" >"${script}"

# The runner: the script as github-script runs it, an async function of
# github, context and require, with a client that pages through the comments
# in COMMENTS and prints what it was asked to do.
readonly runner="${workdir}/runner.cjs"
cat >"${runner}" <<'RUNNER'
const fs = require('fs')
const calls = []
const github = {
  paginate: async () => JSON.parse(process.env.COMMENTS),
  rest: {
    issues: {
      listComments: () => {},
      updateComment: async ({ comment_id, body }) => calls.push(`update ${comment_id} ${JSON.stringify(body)}`),
      createComment: async ({ body }) => calls.push(`create ${JSON.stringify(body)}`),
    },
  },
}
const context = { repo: { owner: 'o', repo: 'r' }, issue: { number: 1 } }
const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor
new AsyncFunction('github', 'context', 'require', fs.readFileSync(process.argv[2], 'utf8'))(
  github,
  context,
  require,
).then(() => console.log(calls.join(', ')))
RUNNER
printf '<!-- coverage-report -->\n## Tests\n' >"${workdir}/ci-report.md"

count_case
if ! grep -q 'updateComment' "${script}"; then
  fail_case "the upsert step" "its script was not found in ci.yml: $(cat "${script}")"
fi

# upsert runs the step's script over the comments given, as JSON, for a run
# of the head this test calls new.
#   upsert <comments>
upsert() {
  run_in "${workdir}" env COMMENT=ci-report.md COMMENTS="$1" DOCS_ONLY=false HEAD_SHA="${new}" \
    node "${runner}" "${script}"
}

# docs_only runs it as a docs-only run of that head does, with no report to
# read.
#   docs_only <comments>
docs_only() {
  run_in "${workdir}" env COMMENT=no-report.md COMMENTS="$1" DOCS_ONLY=true HEAD_SHA="${new}" \
    node "${runner}" "${script}"
}

readonly bot='{"login":"github-actions[bot]","type":"Bot"}'
readonly person='{"login":"someone","type":"User"}'
readonly marked='<!-- coverage-report -->'
readonly old=1111111111111111111111111111111111111111
readonly new=2222222222222222222222222222222222222222
readonly measured_old="${marked}\\n<!-- coverage-head: ${old} -->"

expect_output pass "the bot's comment, updated" "update 2" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"hi\"},{\"id\":2,\"user\":${bot},\"body\":\"${marked} old\"}]"
expect_output pass "no comment yet, one posted" "create" upsert "[{\"id\":1,\"user\":${person},\"body\":\"hi\"}]"
expect_output pass "a person's comment quoting the marker, left alone" "create" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"see ${marked}\"}]"
expect_output pass "a person's quote before the bot's comment" "update 2" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"${marked}\"},{\"id\":2,\"user\":${bot},\"body\":\"${marked}\"}]"

expect_output pass "a report names the head it measured" \
  "update 2 \"${marked}\\n## Tests\\n\\n<!-- coverage-head: ${new} -->\\n\"" \
  upsert "[{\"id\":2,\"user\":${bot},\"body\":\"${marked} old\"}]"

readonly note="Coverage was not measured for ${new}, which changes the docs alone; the last report, for ${old}, is in this comment's edit history."
expect_output pass "a docs-only run, the report it leaves noted" \
  "update 2 \"${measured_old}\\n${note}\"" \
  docs_only "[{\"id\":1,\"user\":${person},\"body\":\"hi\"},{\"id\":2,\"user\":${bot},\"body\":\"${measured_old}\\n## Tests\"}]"
expect_output pass "a docs-only run after another, the head last measured kept" \
  "update 2 \"${measured_old}\\n${note}\"" \
  docs_only "[{\"id\":2,\"user\":${bot},\"body\":\"${measured_old}\\nCoverage was not measured for 3333333333333333333333333333333333333333, which changes the docs alone.\"}]"
expect_output pass "a docs-only run over a report that names no head" \
  "update 2 \"${marked}\\nCoverage was not measured for ${new}, which changes the docs alone; the last report, for an earlier head, is in this comment's edit history.\"" \
  docs_only "[{\"id\":2,\"user\":${bot},\"body\":\"${marked}\\n## Tests\"}]"
expect_exit pass "a docs-only run with no comment yet" docs_only "[{\"id\":1,\"user\":${person},\"body\":\"hi\"}]"
count_case
if [[ -n "$(cat "${case_output}")" ]]; then
  fail_case "a docs-only run with no comment yet" "it posted: $(cat "${case_output}")"
fi

# The job runs for a docs-only change, whose summary is skipped, and fetches
# no report there, since the summary uploaded none.
readonly job="${workdir}/job.yml"
awk '/^  coverage-comment:$/ { job = 1; print; next } job && /^  [^ #]/ { exit } job { print }' "${workflow}" >"${job}"
count_case
if ! grep -q "needs.changes.outputs.docs == 'true'" "${job}"; then
  fail_case "the job for a docs-only change" "its condition does not let a docs-only run through: $(grep -A6 '^    if:' "${job}" || true)"
fi
count_case
if ! grep -B2 'name: ci-report' "${job}" | grep -q "needs.changes.outputs.docs != 'true'"; then
  fail_case "the job for a docs-only change" "it would fetch the report a docs-only run never uploads"
fi

finish_tests
