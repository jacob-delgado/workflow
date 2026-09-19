import { defineConfig } from '@hey-api/openapi-ts'

// Generates the frontend API layer from the repo-root OpenAPI contract
// (../api/openapi.yaml — the single source of truth, also consumed by the Go
// oapi-codegen). Output is the generated SDK + types + TanStack Query options +
// zod runtime-validation schemas under src/api/generated (git-tracked).
export default defineConfig({
  input: '../api/openapi.yaml',
  output: 'src/api/generated',
  plugins: [
    '@hey-api/client-fetch',
    '@hey-api/typescript',
    // Response-only zod validation: every response is runtime-validated against
    // the contract, catching server/client drift. Request validation is off —
    // the Go server already validates every request against the same spec, and
    // a client-side request validator would surface a form-input error as a
    // false failure before the request is even sent.
    { name: '@hey-api/sdk', validator: { request: false, response: true } },
    // The generated queryOptions ARE the app's query layer, re-exported through
    // one-line adapters in features/*/*Api.ts. Mutation factories are off: v1
    // has no writes but the config PUT, which is an imperative call.
    { name: '@tanstack/react-query', mutationOptions: false },
    // offset: true keeps the datetime validators on the full RFC 3339 grammar
    // the contract's `format: date-time` permits — the Go server serializes
    // time.Time with a numeric zone offset (e.g. -06:00), not a Z suffix, and
    // the default offset-less validator would reject every timestamp.
    { name: 'zod', dates: { offset: true } },
  ],
})
