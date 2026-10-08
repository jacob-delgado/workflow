import { render, screen } from '@testing-library/react'
import { markShape } from '@/test/marks.tsx'
import { StateMark, type MarkState } from './StateMark.tsx'

const states: MarkState[] = ['not-started', 'in-flight', 'done', 'failed', 'unknown']

test('draws each state in a shape of its own, hidden from assistive tech', () => {
  // Act
  render(
    <ul>
      {states.map((state) => (
        <li key={state}>
          <StateMark state={state} />
        </li>
      ))}
    </ul>,
  )

  // Assert
  const shapes = screen.getAllByRole('listitem').map(markShape)
  expect(shapes).not.toContain('')
  expect(new Set(shapes).size).toBe(states.length)
})
