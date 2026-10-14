import css from './index.css?raw'

// The color tokens are the page's contract with WCAG's contrast floor. axe
// checks the text it can see, but not a disabled control's, nor an icon's,
// nor a field's border, so the disabled treatment, the field boundary and the
// identity hues are checked here, from the tokens themselves, in both themes.

const themes = [
  ['dark', ':root'],
  ['light', ":root[data-theme='light']"],
] as const

// The systems the product ties together, each with its own hue — the terminal
// spine's (internal/tui/spine.go) — and your own tasks', carrying identity:
// whose a thing is.
const identities = ['--jira', '--git', '--forge', '--messaging', '--taskwarrior'] as const

// What already speaks for state and for controls, which an identity hue must
// never be mistaken for: the three status lights and the control accent.
const reserved = ['--success', '--warning', '--destructive', '--primary'] as const

// Two hues read as one family below this many degrees apart in OKLCH, the
// space where hue is perceptually even: about one named step of its wheel
// (amber at 80° to yellow at 110°, green at 145° to teal at 190°).
const minimumHueSeparation = 30

// Below this chroma a color is near gray and its hue angle means little, so a
// hue held apart by angle must have at least this much of it.
const minimumChroma = 0.07

// The grounds a field or a hue sits on: the page, a card on it, and the list
// beside a view.
const grounds = ['--background', '--card', '--list'] as const

// tokensOf reads the custom properties the first rule for selector declares.
function tokensOf(selector: string): Map<string, string> {
  const start = css.indexOf(`${selector} {`)
  const block = css.slice(start, css.indexOf('}', start))

  return new Map(
    [...block.matchAll(/(--[a-z-]+):\s*(#[0-9a-f]{6});/gi)].map(([, name, value]) => [
      name ?? '',
      value ?? '',
    ]),
  )
}

// linearChannels is a color's red, green and blue, each linearized from sRGB.
function linearChannels(hex: string): [number, number, number] {
  const [red = 0, green = 0, blue = 0] = [1, 3, 5].map((at) => {
    const channel = Number.parseInt(hex.slice(at, at + 2), 16) / 255

    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
  })

  return [red, green, blue]
}

// luminance is a color's relative luminance, as WCAG defines it.
function luminance(hex: string): number {
  const [red, green, blue] = linearChannels(hex)

  return 0.2126 * red + 0.7152 * green + 0.0722 * blue
}

// contrast is the WCAG contrast ratio between two colors.
function contrast(first: string, second: string): number {
  const [lighter, darker] = [luminance(first), luminance(second)].sort((a, b) => b - a)

  return ((lighter ?? 0) + 0.05) / ((darker ?? 0) + 0.05)
}

// oklch is a color's chroma and hue angle in OKLCH (Björn Ottosson's OKLab,
// in polar form).
function oklch(hex: string): { chroma: number; hue: number } {
  const [red, green, blue] = linearChannels(hex)
  const long = Math.cbrt(0.4122214708 * red + 0.5363325363 * green + 0.0514459929 * blue)
  const medium = Math.cbrt(0.2119034982 * red + 0.6806995451 * green + 0.1073969566 * blue)
  const short = Math.cbrt(0.0883024619 * red + 0.2817188376 * green + 0.6299787005 * blue)
  const a = 1.9779984951 * long - 2.428592205 * medium + 0.4505937099 * short
  const b = 0.0259040371 * long + 0.7827717662 * medium - 0.808675766 * short

  return { chroma: Math.hypot(a, b), hue: ((Math.atan2(b, a) * 180) / Math.PI + 360) % 360 }
}

// hueSeparation is how far apart two colors' hues sit on the OKLCH wheel, in
// degrees, the short way round.
function hueSeparation(first: string, second: string): number {
  const apart = Math.abs(oklch(first).hue - oklch(second).hue)

  return Math.min(apart, 360 - apart)
}

// color is the token's value in a theme, failing the test when the theme does
// not declare it.
function color(tokens: Map<string, string>, name: string): string {
  const value = tokens.get(name) ?? 'undeclared'
  expect(value, `the theme declares ${name}`).toMatch(/^#[0-9a-f]{6}$/i)

  return value
}

// declared is what a rule's block declares for a property, its whitespace
// run together, or undefined where the block leaves it out.
function declared(block: string, property: string): string | undefined {
  const escaped = property.replaceAll('-', '\\-')

  return new RegExp(`(?:^|[\\s;{])${escaped}:\\s*([^;]+);`)
    .exec(block)?.[1]
    ?.replaceAll(/\s+/g, ' ')
    .trim()
}

// families is a font stack's faces in order, each without its quotes.
function families(stack: string | undefined): string[] {
  return (stack ?? '').split(',').map((face) => face.trim().replaceAll(/^'|'$/g, ''))
}

// blocksOf is the body of every rule the stylesheet opens with opening, in
// order.
function blocksOf(opening: string): string[] {
  return css
    .split(opening)
    .slice(1)
    .map((rest) => rest.slice(0, rest.indexOf('}')))
}

// The page's own face: Recursive, its latin mono file alone, the one variable
// font whose MONO axis sets code apart from prose without a second family.
const face = 'Recursive Variable'

test('the page loads one face, Recursive’s latin mono file, swapped in as it arrives', () => {
  // Act
  const faces = blocksOf('@font-face {')

  // Assert
  expect(faces).toHaveLength(1)
  const [only = ''] = faces
  expect(families(declared(only, 'font-family'))).toEqual([face])
  expect(declared(only, 'src')).toMatch(
    /^url\('[^']*\/recursive-latin-mono-normal\.woff2'\) format\('woff2-variations'\)$/,
  )
  expect(declared(only, 'font-display')).toBe('swap')
  expect(declared(only, 'font-weight')).toBe('300 1000')
  expect(declared(only, 'unicode-range')).toMatch(/^U\+0000-00FF,/)
})

test.each([
  ['--font-sans', "'MONO' 0", 'system-ui', 'sans-serif'],
  ['--font-mono', "'MONO' 1", 'ui-monospace', 'monospace'],
])(
  '%s is Recursive at %s, falling back to the platform’s own face',
  (font, axis, platform, generic) => {
    // Arrange
    const [theme = ''] = blocksOf('@theme {')

    // Act
    const stack = families(declared(theme, font))

    // Assert
    expect(stack[0]).toBe(face)
    expect(stack[1]).toBe(platform)
    expect(stack.at(-1)).toBe(generic)
    expect(declared(theme, `${font}--font-variation-settings`)).toBe(axis)
  },
)

// selectionTokens are the tokens the page's ::selection rule draws selected
// text in and on, or a name no theme declares when either is not a token.
function selectionTokens(): { ground: string; text: string } {
  const start = css.indexOf('::selection {')
  const block = css.slice(start, css.indexOf('}', start))

  return {
    ground: /background-color:\s*var\((--[a-z-]+)\);/.exec(block)?.[1] ?? 'a mix, not a token',
    text: /[\s;{]color:\s*var\((--[a-z-]+)\);/.exec(block)?.[1] ?? 'a mix, not a token',
  }
}

// Selected text is drawn on a solid ground of its own, so it reads the same
// on every surface it is selected on, whatever that surface's fill; a primary
// fill, which is that ground, swaps the two.
test.each(themes)(
  'selected text holds 4.5:1 on its selection ground in the %s theme',
  (_, selector) => {
    // Arrange
    const tokens = tokensOf(selector)
    const { ground, text } = selectionTokens()

    // Act
    const ratio = contrast(color(tokens, text), color(tokens, ground))

    // Assert
    expect(ratio).toBeGreaterThanOrEqual(4.5)
  },
)

test.each(themes)(
  "a disabled control's text holds 4.5:1 on its fill in the %s theme",
  (_, selector) => {
    // Arrange
    const tokens = tokensOf(selector)

    // Act
    const ratio = contrast(
      tokens.get('--disabled-foreground') ?? '',
      tokens.get('--disabled') ?? '',
    )

    // Assert
    expect(ratio).toBeGreaterThanOrEqual(4.5)
  },
)

describe.each(themes)('in the %s theme', (_, selector) => {
  test.each([
    ['--destructive', '--destructive-surface'],
    ['--success', '--success-surface'],
    ['--warning', '--warning-surface'],
  ])('%s holds 4.5:1 as text on its notice ground, %s', (state, surface) => {
    // Arrange
    const tokens = tokensOf(selector)

    // Act
    const ratio = contrast(color(tokens, state), color(tokens, surface))

    // Assert
    expect(ratio).toBeGreaterThanOrEqual(4.5)
  })

  // A row of the issue or task list fills with --accent while it is hovered
  // or selected, and its marks stay drawn on it; what a row says in words
  // about a refusal sits on its notice ground instead.
  test.each(['--success', '--warning', '--destructive'])(
    '%s holds 3:1 as a mark on a hovered or selected row, --accent',
    (state) => {
      // Arrange
      const tokens = tokensOf(selector)

      // Act
      const ratio = contrast(color(tokens, state), color(tokens, '--accent'))

      // Assert
      expect(ratio).toBeGreaterThanOrEqual(3)
    },
  )

  test.each(['--activity-low', '--activity-mid', '--activity-high'])(
    "a calendar day's number, and the dots under it, hold 4.5:1 on %s",
    (level) => {
      // Arrange
      const tokens = tokensOf(selector)

      // Act
      const ratio = contrast(color(tokens, '--foreground'), color(tokens, level))

      // Assert
      expect(ratio).toBeGreaterThanOrEqual(4.5)
    },
  )

  test.each(['--tonal', '--tonal-hover'])(
    "a secondary button's words hold 4.5:1 on its %s fill",
    (fill) => {
      // Arrange
      const tokens = tokensOf(selector)

      // Act
      const ratio = contrast(color(tokens, '--tonal-foreground'), color(tokens, fill))

      // Assert
      expect(ratio).toBeGreaterThanOrEqual(4.5)
    },
  )

  test.each(grounds)(
    "a pressed filter chip's --tonal-foreground edge holds 3:1 on %s",
    (beside) => {
      // Arrange
      const tokens = tokensOf(selector)

      // Act
      const ratio = contrast(color(tokens, '--tonal-foreground'), color(tokens, beside))

      // Assert
      expect(ratio).toBeGreaterThanOrEqual(3)
    },
  )

  test.each(grounds)("a field's --input boundary holds 3:1 on %s", (ground) => {
    // Arrange
    const tokens = tokensOf(selector)

    // Act
    const ratio = contrast(color(tokens, '--input'), color(tokens, ground))

    // Assert
    expect(ratio).toBeGreaterThanOrEqual(3)
  })

  test.each(identities)('%s reads as text on the page, a card and the list', (identity) => {
    // Arrange
    const tokens = tokensOf(selector)
    const hue = color(tokens, identity)

    // Act
    const ratios = grounds.map((ground) => contrast(hue, color(tokens, ground)))

    // Assert
    expect(Math.min(...ratios)).toBeGreaterThanOrEqual(4.5)
  })

  test.each(identities)('%s holds 3:1 as an icon on the active rail item', (identity) => {
    // Arrange
    const tokens = tokensOf(selector)

    // Act
    const ratio = contrast(color(tokens, identity), color(tokens, '--accent'))

    // Assert
    expect(ratio).toBeGreaterThanOrEqual(3)
  })

  test.each(identities)(
    '%s is never mistaken for a status light, the control accent or another system',
    (identity) => {
      // Arrange
      const tokens = tokensOf(selector)
      const hue = color(tokens, identity)
      const others = [...reserved, ...identities.filter((other) => other !== identity)]

      // Act
      const separations = others.map((other) => hueSeparation(hue, color(tokens, other)))

      // Assert
      expect(oklch(hue).chroma).toBeGreaterThanOrEqual(minimumChroma)
      expect(Math.min(...separations)).toBeGreaterThanOrEqual(minimumHueSeparation)
    },
  )
})
