// Resolve the theme before first paint so there is no dark-then-light flash.
// The React store keeps it in step afterwards; this only sets the opening value
// from the saved choice, resolving "system" against the OS. index.html loads it
// as a classic script in its head, which runs before the page paints; it is a
// file of its own because the server's content policy runs no script written
// into the page.
;(function () {
  function systemDark() {
    try {
      return window.matchMedia('(prefers-color-scheme: dark)').matches
    } catch {
      return true
    }
  }

  let dark = systemDark()
  try {
    const stored = localStorage.getItem('workflow-theme')
    if (stored === 'light' || stored === 'dark') {
      dark = stored === 'dark'
    }
  } catch {
    // Storage blocked — fall back to the system preference, as the store does.
  }
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
})()
