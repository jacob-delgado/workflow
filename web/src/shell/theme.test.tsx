import { renderHook, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import App from '@/App.tsx'
import { listenerCount } from '@/test/matchMedia.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import indexHtml from '../../index.html?raw'
import { themeStorageKey } from './themeKey.ts'

// prePaint is the script index.html runs in its head, before the page paints.
const prePaint =
  new DOMParser().parseFromString(indexHtml, 'text/html').querySelector('head > script')
    ?.textContent ?? ''

// themeBeforePaint is the theme the pre-paint script resolves, run as the
// browser runs it: a classic script in the page.
function themeBeforePaint(): string | undefined {
  delete document.documentElement.dataset.theme
  const script = document.createElement('script')
  script.textContent = prePaint
  document.head.append(script)
  script.remove()

  return document.documentElement.dataset.theme
}

// themeOnceLoaded is the theme the app applies once it loads, its store read
// afresh from storage as a page load reads it.
async function themeOnceLoaded(): Promise<string | undefined> {
  delete document.documentElement.dataset.theme
  vi.resetModules()
  const { useApplyTheme } = await import('./useApplyTheme.ts')
  renderHook(() => {
    useApplyTheme()
  })

  return document.documentElement.dataset.theme
}

// preferDark has the OS ask for a dark scheme.
function preferDark(): void {
  const light = window.matchMedia.bind(window)
  vi.stubGlobal('matchMedia', (query: string) => Object.assign(light(query), { matches: true }))
}

test('the theme toggle cycles system, light, dark and remembers the choice', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)

  // Act: it opens following the system
  // Assert
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()

  // Act: click through the cycle
  // Assert: each step advances the choice and persists it
  await user.click(screen.getByRole('button', { name: /theme: system/i }))
  expect(screen.getByRole('button', { name: /theme: light/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('light')

  await user.click(screen.getByRole('button', { name: /theme: light/i }))
  expect(screen.getByRole('button', { name: /theme: dark/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('dark')

  await user.click(screen.getByRole('button', { name: /theme: dark/i }))
  expect(screen.getByRole('button', { name: /theme: system/i })).toBeTruthy()
  expect(localStorage.getItem(themeStorageKey)).toBe('system')
})

test('the theme toggle shows the choice it holds on hover, and follows it', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)
  const toggle = screen.getByRole('button', { name: /^Theme: System/ })
  const before = toggle.getAttribute('title')

  // Act
  await user.click(toggle)

  // Assert
  expect(before).toBe('Theme: System')
  expect(toggle.getAttribute('title')).toBe('Theme: Light')
})

test('following the system registers one OS listener and drops it when set to light', async () => {
  // Arrange
  const user = userEvent.setup()
  renderWithClient(<App />)

  // Act: it opens following the system
  // Assert: exactly one OS-change listener is registered to follow it
  expect(listenerCount()).toBe(1)

  // Act: switch off "system" to a forced choice
  await user.click(screen.getByRole('button', { name: /theme: system/i }))

  // Assert: the OS listener is removed — no leak, and no stale follow while forced
  expect(listenerCount()).toBe(0)
})

test.each([
  ['light', 'a light OS', 'light'],
  ['dark', 'a light OS', 'dark'],
  ['system', 'a light OS', 'light'],
  ['chartreuse', 'a light OS', 'light'],
  ['light', 'a dark OS', 'light'],
  ['dark', 'a dark OS', 'dark'],
  ['system', 'a dark OS', 'dark'],
  ['chartreuse', 'a dark OS', 'dark'],
])(
  'a saved %s under %s is the %s theme before paint and once the app loads',
  async (saved, os, theme) => {
    // Arrange
    localStorage.setItem(themeStorageKey, saved)
    if (os === 'a dark OS') {
      preferDark()
    }

    // Act
    const beforePaint = themeBeforePaint()
    const onceLoaded = await themeOnceLoaded()

    // Assert
    expect({ beforePaint, onceLoaded }).toEqual({ beforePaint: theme, onceLoaded: theme })
  },
)
