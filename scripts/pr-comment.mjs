// pr-comment.mjs — render the pull request's CI comment: each suite's test
// counts, then the coverage and its delta against main's baseline.
//
// Usage: node scripts/pr-comment.mjs <pr-coverage.json> <baseline.json> <counts-dir>
//
// <counts-dir> holds one test-counts.sh line per suite that reported, named for
// the suite (go-unit.json, web-unit.json, e2e.json, e2e-server.json). A suite
// with no file, or with null counts — its job failed before it could count —
// reads as dashes, never as zeros. A missing coverage summary or baseline
// reads as n/a and no delta, as when the Test job failed.
//
// The CI job posts what this prints; keeping the markdown here, rather than in
// the workflow, is what lets scripts/pr-comment_test.sh check it.
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const marker = '<!-- coverage-report -->'

// The suites in the order the comment lists them, each with its counts file.
const suites = [
  ['Go unit', 'go-unit.json'],
  ['Web unit (vitest)', 'web-unit.json'],
  ['E2E', 'e2e.json'],
  ['E2E (server)', 'e2e-server.json'],
]

const coverageRows = [
  ['Statements', 'statements'],
  ['Conditions (gobco)', 'branch'],
]

function readJSON(path) {
  if (!existsSync(path)) {
    return null
  }

  try {
    return JSON.parse(readFileSync(path, 'utf8'))
  } catch {
    return null
  }
}

const count = (n) => (n === null || n === undefined ? '—' : String(n))
const pct = (n) => (n === null || n === undefined ? 'n/a' : `${Number(n).toFixed(1)}%`)

function delta(current, previous) {
  if (current === null || current === undefined || previous === null || previous === undefined) {
    return '—'
  }

  const change = Math.round((current - previous) * 10) / 10

  return change === 0 ? '±0' : (change > 0 ? '+' : '') + change.toFixed(1)
}

function testsTable(countsDir) {
  const rows = suites.map(([label, file]) => {
    const counts = readJSON(join(countsDir, file)) ?? {}

    return `| ${label} | ${count(counts.passed)} | ${count(counts.skipped)} | ${count(counts.failed)} |`
  })

  return ['## Tests', '', '| Suite | Passed | Skipped | Failed |', '| --- | ---: | ---: | ---: |', ...rows]
}

function coverageTable(pr, base) {
  const rows = coverageRows.map(
    ([label, key]) => `| ${label} | ${pct(pr?.[key])} | ${delta(pr?.[key], base?.[key])} |`,
  )

  return [
    '## Coverage',
    '',
    '| Metric | Coverage | Δ vs main |',
    '| --- | --- | --- |',
    ...rows,
    '',
    '_The floors live in the gates, not here — `COVERAGE_MIN` and ' +
      '`BRANCH_COVERAGE_MIN` in `Taskfile.yml`. A number restated in prose drifts ' +
      'from the one enforced._',
    '',
    '_Conditions counts each operand separately: an `if a && b` is only covered ' +
      'once `a` and `b` have each been seen true and false. gobco reads every ' +
      'package in this module but the build-tagged twins ' +
      '`scripts/gobco-report.sh` lists; `task cover:branch` fails if another ' +
      'drops out unlisted._',
  ]
}

const [prPath, basePath, countsDir] = process.argv.slice(2)
if (countsDir === undefined) {
  process.stderr.write('usage: pr-comment.mjs <pr-coverage.json> <baseline.json> <counts-dir>\n')
  process.exit(2)
}

const body = [
  marker,
  ...testsTable(countsDir),
  '',
  ...coverageTable(readJSON(prPath), readJSON(basePath)),
]
process.stdout.write(`${body.join('\n')}\n`)
