// The key the theme choice is saved under. It sits apart from the store, with
// nothing imported and nothing read as it loads, so the test setup and the
// Playwright specs name it without loading zustand or touching storage. The
// pre-paint script in index.html cannot import it and spells it itself;
// theme.test.tsx holds that script to this key.
export const themeStorageKey = 'workflow-theme'
