/// <reference types="vite/client" />

interface ImportMetaEnv {
  // Set by `task web:mockup` to run the UI against fixture data with no backend.
  readonly VITE_MOCK?: string
}
