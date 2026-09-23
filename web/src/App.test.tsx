import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import App from './App.tsx'
import { FakeEventSource } from './test/fakeEventSource.ts'

test('shows the sections and opens on the Issues view', () => {
  // Arrange
  render(<App />)

  // Act
  const nav = screen.getByRole('navigation', { name: /sections/i })

  // Assert
  expect(nav).toBeTruthy()
  expect(screen.getByRole('heading', { level: 1, name: /issues/i })).toBeTruthy()
})

test('switches the view when another section is chosen', async () => {
  // Arrange
  const user = userEvent.setup()
  render(<App />)

  // Act
  await user.click(screen.getByRole('button', { name: /branch/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 1, name: /branch/i })).toBeTruthy()
})

// serveHealth answers the health read the shell makes on mount.
function serveHealth(dryRun: boolean) {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.resolve(Response.json({ version: '1.2.3', dry_run: dryRun }))),
  )
}

test('a dry-run server shows the read-only banner', async () => {
  // Arrange
  serveHealth(true)

  // Act
  render(<App />)

  // Assert
  await waitFor(() => {
    const banner = screen
      .getAllByRole('status')
      .find((region) => /every write is held back/i.test(region.textContent))
    expect(banner).toBeDefined()
  })
})

test('shows the server version in the header', async () => {
  // Arrange
  serveHealth(false)

  // Act
  render(<App />)

  // Assert
  const banner = screen.getByRole('banner')
  expect(await within(banner).findByText('1.2.3')).toBeTruthy()
})

test('shows no read-only banner when the server writes', async () => {
  // Arrange
  serveHealth(false)

  // Act
  render(<App />)

  // Assert
  await screen.findByText('1.2.3')
  expect(screen.queryByText(/every write is held back/i)).toBeNull()
})

test('drops the read-only banner when the stream comes back from a server that writes', async () => {
  // Arrange
  // The server restarts on its fixed port, this time without --dry-run; the
  // stream drops and reconnects to it.
  const dryRuns = [true, false]
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(Response.json({ version: '1.2.3', dry_run: dryRuns.shift() ?? false })),
    ),
  )
  render(<App />)
  await screen.findByText(/every write is held back/i)
  act(() => {
    FakeEventSource.latest().emit('error', '')
  })

  // Act
  act(() => {
    FakeEventSource.latest().emit('open', '')
  })

  // Assert
  await waitFor(() => {
    expect(screen.queryByText(/every write is held back/i)).toBeNull()
  })
})
