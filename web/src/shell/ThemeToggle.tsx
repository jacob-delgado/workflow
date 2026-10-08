import { Monitor, Moon, Sun } from 'lucide-react'
import { useThemeStore, type ThemeChoice } from './themeStore.ts'

// How each choice presents in the header: its icon and the word a screen reader
// announces. The map sits beside the toggle, the one place a choice is turned
// into UI.
const choiceMeta: Record<ThemeChoice, { label: string; Icon: typeof Sun }> = {
  system: { label: 'System', Icon: Monitor },
  light: { label: 'Light', Icon: Sun },
  dark: { label: 'Dark', Icon: Moon },
}

// ThemeToggle cycles system → light → dark from a single header button. Icon-
// only, so it carries its accessible name and the current choice in aria-label,
// and shows the choice on hover in its title, as the rail's buttons show theirs.
export function ThemeToggle() {
  const choice = useThemeStore((state) => state.choice)
  const cycleChoice = useThemeStore((state) => state.cycleChoice)
  const { label, Icon } = choiceMeta[choice]

  return (
    <button
      type="button"
      onClick={cycleChoice}
      aria-label={`Theme: ${label}. Change theme.`}
      title={`Theme: ${label}`}
      className="flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <Icon aria-hidden className="size-4" />
    </button>
  )
}
