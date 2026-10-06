// A key the page binds is one character as typed — c, C, /, [ — which a
// keydown reports as its key. The terminal's other names (space, ctrl+e,
// shift+tab, pgup) are left to the browser, which keeps its combinations for
// itself.

// pressable reports a key the page can bind: one character. A key a keydown
// names in words — Enter, Dead, F5 — is longer.
export function pressable(key: string): boolean {
  return key.length === 1
}

// ariaShortcut names a key as aria-keyshortcuts does: an upper-case letter is
// typed with Shift, every other character as itself.
function ariaShortcut(key: string): string {
  return /^[A-Z]$/.test(key) ? `Shift+${key}` : key
}

// ariaShortcuts is the aria-keyshortcuts value for an action's keys, or
// undefined where the page binds none of them.
export function ariaShortcuts(keys: string[]): string | undefined {
  const named = keys.filter(pressable).map(ariaShortcut)

  return named.length === 0 ? undefined : named.join(' ')
}

// takesTyping reports a press made where a character is typed or chosen — a
// field, a select, an editable region, or the comment composer and its bar —
// where no single key may act.
export function takesTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false
  }

  return (
    target.isContentEditable ||
    target.closest('input, textarea, select, [contenteditable], [data-typing]') !== null
  )
}

// plainKey is the character a press typed with no Control, Command or Alt
// held, or undefined for any other press: a combination belongs to the
// browser, and a press mid-composition to the input method.
export function plainKey(event: KeyboardEvent): string | undefined {
  if (event.ctrlKey || event.metaKey || event.altKey || event.isComposing) {
    return undefined
  }

  return pressable(event.key) ? event.key : undefined
}

// onMac reports a Mac, where Control+K in a text field is the field's own
// key — delete to the end of the line — and Command+K is the page's.
function onMac(): boolean {
  return /mac|iphone|ipad/i.test(navigator.platform)
}

// opensPalette reports Command+K, or Control+K but in a Mac's text field, the
// one combination the page takes from the browser; never mid-composition,
// where the press is the input method's.
export function opensPalette(event: KeyboardEvent): boolean {
  if (event.isComposing || event.altKey || event.key.toLowerCase() !== 'k') {
    return false
  }

  return event.metaKey || (event.ctrlKey && !(onMac() && takesTyping(event.target)))
}
