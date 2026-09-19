/** @type {import('dependency-cruiser').IConfiguration} */
// Frontend import-boundary rules (the JS analogue of the Go depguard rules).
// Run via `depcruise` in `yarn lint`. The generated-SDK boundary rule is added
// with the api layer that introduces the seam.
module.exports = {
  forbidden: [
    {
      name: 'no-circular',
      severity: 'error',
      comment: 'Circular imports make modules impossible to reason about or test in isolation.',
      from: {},
      to: { circular: true },
    },
  ],
  options: {
    tsConfig: { fileName: 'tsconfig.app.json' },
    tsPreCompilationDeps: true,
    // Don't cruise into the generated hey-api SDK: it's machine-emitted and,
    // like prettier/knip/linguist, excluded from our tooling.
    doNotFollow: { path: 'node_modules|^src/api/generated' },
    // Tests legitimately reach across boundaries (they import the unit under
    // test, fakes, etc.); the rules govern the app graph.
    exclude: { path: '\\.(test|spec)\\.tsx?$' },
  },
}
