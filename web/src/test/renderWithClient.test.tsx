import { useQuery } from '@tanstack/react-query'
import { screen } from '@testing-library/react'
import { vi } from 'vitest'
import { renderWithClient } from './renderWithClient.tsx'

// Reads draws one query, counting each time it reads.
function Reads({ onRead }: { onRead: () => void }) {
  const { data } = useQuery({
    queryKey: ['policy'],
    queryFn: () => {
      onRead()

      return 'read'
    },
  })

  return <p>{data ?? 'reading'}</p>
}

test('a query drawn again reads nothing again, under the app’s own policy', async () => {
  // Arrange
  const onRead = vi.fn()
  const view = renderWithClient(<Reads key="first" onRead={onRead} />)
  await screen.findByText('read')

  // Act
  view.rerender(<Reads key="again" onRead={onRead} />)

  // Assert
  expect(screen.getByText('read')).toBeTruthy()
  expect(onRead).toHaveBeenCalledTimes(1)
})
