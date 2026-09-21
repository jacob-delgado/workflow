import { create } from 'zustand'

// The theme choices, in the order the header toggle cycles them: follow the OS,
// then force light, then force dark, then back to following the OS.
const themeChoices = ['system', 'light', 'dark'] as const

export type ThemeChoice = (typeof themeChoices)[number]

// The key the pre-paint script in index.html reads too, so the saved choice
// takes effect before React mounts. Keep the two in step.
const storageKey = 'workflow-theme'

// readStoredChoice recovers the saved choice, defaulting to "system" when there
// is none or storage is blocked (a private window, cleared site data). Exported
// so its restore path can be tested — the store reads it once, at load.
export function readStoredChoice(): ThemeChoice {
  try {
    const stored = localStorage.getItem(storageKey)
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      return stored
    }
  } catch {
    // Storage unavailable — fall through to the default.
  }

  return 'system'
}

// persistChoice remembers the choice for the next visit, ignoring a storage that
// refuses the write rather than failing the toggle.
function persistChoice(choice: ThemeChoice): void {
  try {
    localStorage.setItem(storageKey, choice)
  } catch {
    // Storage unavailable — the choice still applies for this session.
  }
}

interface ThemeState {
  choice: ThemeChoice
  cycleChoice: () => void
}

// The theme preference, persisted to localStorage and shared by the toggle and
// the apply hook. Client-only: the theme never round-trips to the server.
export const useThemeStore = create<ThemeState>((set, get) => ({
  choice: readStoredChoice(),
  cycleChoice: () => {
    const nextIndex = (themeChoices.indexOf(get().choice) + 1) % themeChoices.length
    const next = themeChoices[nextIndex] ?? 'system'
    persistChoice(next)
    set({ choice: next })
  },
}))
