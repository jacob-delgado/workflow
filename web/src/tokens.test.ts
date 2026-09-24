import css from './index.css?raw'

// The color tokens are the page's contract with WCAG's contrast floor. axe
// checks the text it can see, but not a disabled control's, nor an icon's, so
// the disabled treatment and the identity hues are checked here, from the
// tokens themselves, in both themes.

const themes = [
  ['dark', ':root'],
  ['light', ":root[data-theme='light']"],
] as const

// The systems the product ties together, each with its own hue — the terminal
// spine's (internal/tui/spine.go) — carrying identity: whose a thing is.
const identities = ['--jira', '--git', '--forge', '--messaging'] as const

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
  test.each(identities)('%s reads as text on the page and on a card', (identity) => {
    // Arrange
    const tokens = tokensOf(selector)
    const hue = color(tokens, identity)

    // Act
    const ratios = ['--background', '--card'].map((ground) => contrast(hue, color(tokens, ground)))

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
