import { vi } from 'vitest'
import { opensPalette } from './keyNames.ts'

// press is a keydown of key with the modifiers given, made at target.
function press(target: EventTarget, init: KeyboardEventInit): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { key: 'k', bubbles: true, ...init })
  Object.defineProperty(event, 'target', { value: target })

  return event
}

// onPlatform makes the browser report platform for the test's length.
function onPlatform(platform: string) {
  vi.spyOn(navigator, 'platform', 'get').mockReturnValue(platform)
}

afterEach(() => {
  vi.restoreAllMocks()
})

const field = () => document.createElement('input')
const page = () => document.body

test.each([
  {
    name: 'Ctrl+K outside a field on a Mac',
    platform: 'MacIntel',
    at: page,
    init: { ctrlKey: true },
    opens: true,
  },
  {
    name: 'Ctrl+K in a field on a Mac',
    platform: 'MacIntel',
    at: field,
    init: { ctrlKey: true },
    opens: false,
  },
  {
    name: '⌘K in a field on a Mac',
    platform: 'MacIntel',
    at: field,
    init: { metaKey: true },
    opens: true,
  },
  {
    name: 'Ctrl+K in a field elsewhere',
    platform: 'Linux x86_64',
    at: field,
    init: { ctrlKey: true },
    opens: true,
  },
  {
    name: 'Ctrl+K mid-composition',
    platform: 'Linux x86_64',
    at: page,
    init: { ctrlKey: true, isComposing: true },
    opens: false,
  },
  {
    name: '⌘K mid-composition',
    platform: 'MacIntel',
    at: page,
    init: { metaKey: true, isComposing: true },
    opens: false,
  },
  {
    name: 'Ctrl+Alt+K',
    platform: 'Linux x86_64',
    at: page,
    init: { ctrlKey: true, altKey: true },
    opens: false,
  },
])('$name opens the palette: $opens', ({ platform, at, init, opens }) => {
  // Arrange
  onPlatform(platform)
  const event = press(at(), init)

  // Act & Assert
  expect(opensPalette(event)).toBe(opens)
})
