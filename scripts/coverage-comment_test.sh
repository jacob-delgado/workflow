#!/usr/bin/env bash
#
# Tests for the script ci.yml's Coverage comment job runs to post the CI
# report on a pull request: it updates the comment the workflow's own bot
# posted with the report's marker, and posts a new one when there is none,
# leaving alone a comment anybody else wrote, though it quotes the marker.
# The script is read out of ci.yml as the job runs it and run under node with
# a stand-in for the GitHub client that lists the comments a case gives and
# notes what it was asked to update or create. It is skipped where no node is
# on PATH.
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
      updateComment: async ({ comment_id }) => calls.push(`update ${comment_id}`),
      createComment: async () => calls.push('create'),
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

# upsert runs the step's script over the comments given, as JSON.
#   upsert <comments>
upsert() {
  run_in "${workdir}" env COMMENT=ci-report.md COMMENTS="$1" node "${runner}" "${script}"
}

readonly bot='{"login":"github-actions[bot]","type":"Bot"}'
readonly person='{"login":"someone","type":"User"}'
readonly marked='<!-- coverage-report -->'

expect_output pass "the bot's comment, updated" "update 2" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"hi\"},{\"id\":2,\"user\":${bot},\"body\":\"${marked} old\"}]"
expect_output pass "no comment yet, one posted" "create" upsert "[{\"id\":1,\"user\":${person},\"body\":\"hi\"}]"
expect_output pass "a person's comment quoting the marker, left alone" "create" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"see ${marked}\"}]"
expect_output pass "a person's quote before the bot's comment" "update 2" \
  upsert "[{\"id\":1,\"user\":${person},\"body\":\"${marked}\"},{\"id\":2,\"user\":${bot},\"body\":\"${marked}\"}]"

finish_tests
