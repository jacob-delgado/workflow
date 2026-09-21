import { useEffect } from 'react'
import { useThemeStore } from './themeStore.ts'

const darkQuery = '(prefers-color-scheme: dark)'

// useApplyTheme writes the resolved theme onto the document and, while the
// choice is "system", follows the OS as it changes. The pre-paint script in
// index.html sets the opening value, so this only keeps it current — mounted
// once, at the app shell.
export function useApplyTheme(): void {
  const choice = useThemeStore((state) => state.choice)

  useEffect(() => {
    const apply = () => {
      const dark = choice === 'system' ? window.matchMedia(darkQuery).matches : choice === 'dark'
      document.documentElement.dataset.theme = dark ? 'dark' : 'light'
    }

    apply()

    if (choice !== 'system') {
      return
    }

    const media = window.matchMedia(darkQuery)
    media.addEventListener('change', apply)

    return () => {
      media.removeEventListener('change', apply)
    }
  }, [choice])
}
