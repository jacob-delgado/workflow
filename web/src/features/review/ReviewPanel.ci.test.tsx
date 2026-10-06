import { render, screen, within } from '@testing-library/react'
import type { Ci } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { ReviewPanel } from './ReviewPanel.tsx'

// reviewing is an open pull request whose CI stands as ci says.
function reviewing(ci: Ci) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      review: {
        found: true,
        announced: false,
        pull: {
          number: 128,
          url: 'https://forge.example.com/pull/128',
          title: 'Redact tokens in the request log',
          state: 'open',
          draft: false,
          approvals: 1,
          changes_requested: false,
          mergeable: 'clean',
        },
        ci,
      },
    }),
  })
}

test('heads a pipeline the forge counts no checks in by how it stands, not "0 of 0"', () => {
  // Arrange
  // A GitLab pipeline: one check for the whole pipeline, and no counts.
  reviewing({
    state: 'running',
    total: 0,
    done: 0,
    failed: 0,
    checks: [{ name: 'pipeline', state: 'running', url: '' }],
  })

  // Act
  render(<ReviewPanel />)

  // Assert
  // The state is an element apart, which a browser names after a space and the
  // test's DOM without one; the dot before it is never named.
  expect(screen.getByRole('heading', { level: 3, name: /^CI checks\s?running$/ })).toBeTruthy()
  expect(screen.queryByText(/0 of 0/)).toBeNull()
})

test('says no checks are reported, beside the unknown mark, rather than draw an empty list', () => {
  // Arrange
  reviewing({ state: 'none', total: 0, done: 0, failed: 0, checks: [] })

  // Act
  render(<ReviewPanel />)

  // Assert
  const section = screen.getByRole('region', { name: /^CI checks/ })
  const said = within(section).getByText('No checks reported.')
  expect(markShape(said)).toBe(drawnMark('unknown'))
  expect(within(section).queryByRole('list')).toBeNull()
  expect(within(section).queryByText(/0 of 0/)).toBeNull()
})
