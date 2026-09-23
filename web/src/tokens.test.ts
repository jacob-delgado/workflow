import css from './index.css?raw'

// The color tokens are the page's contract with WCAG's contrast floor. axe
// checks the text it can see, but not a disabled control's, so the disabled
// treatment is checked here, from the tokens themselves, in both themes.

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

// luminance is a color's relative luminance, as WCAG defines it.
function luminance(hex: string): number {
  const channels = [1, 3, 5].map((at) => Number.parseInt(hex.slice(at, at + 2), 16) / 255)
  const [red = 0, green = 0, blue = 0] = channels.map((channel) =>
    channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
  )

  return 0.2126 * red + 0.7152 * green + 0.0722 * blue
}

// contrast is the WCAG contrast ratio between two colors.
function contrast(first: string, second: string): number {
  const [lighter, darker] = [luminance(first), luminance(second)].sort((a, b) => b - a)

  return ((lighter ?? 0) + 0.05) / ((darker ?? 0) + 0.05)
}

test.each([
  ['dark', ':root'],
  ['light', ":root[data-theme='light']"],
])("a disabled control's text holds 4.5:1 on its fill in the %s theme", (_, selector) => {
  // Arrange
  const tokens = tokensOf(selector)

  // Act
  const ratio = contrast(tokens.get('--disabled-foreground') ?? '', tokens.get('--disabled') ?? '')

  // Assert
  expect(ratio).toBeGreaterThanOrEqual(4.5)
})
