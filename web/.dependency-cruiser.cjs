/** @type {import('dependency-cruiser').IConfiguration} */
// Frontend import-boundary rules (the JS analogue of the Go depguard rules).
// Run via `depcruise` in `yarn lint`.
module.exports = {
  forbidden: [
    {
      name: 'no-circular',
      severity: 'error',
      comment: 'Circular imports make modules impossible to reason about or test in isolation.',
      from: {},
      to: { circular: true },
    },
    {
      name: 'sdk-only-through-api',
      severity: 'error',
      comment:
        'Value imports of the generated SDK go through src/api or a feature *Api.ts wrapper ' +
        '(an area within a feature, such as settings/people, keeps its own), so a component ' +
        'never calls the server directly; types may be imported anywhere. src/dev is the ' +
        "mockup's stand-in for the server, which reads requests with the contract's schemas " +
        'and calls nothing.',
      from: {
        pathNot: [
          '^src/(api/|dev/|features/[^/]+/[^/]+Api\\.ts$)',
          '^src/features/[^/]+/[^/]+/[^/]+Api\\.ts$',
        ],
      },
      to: { path: '^src/api/generated/', dependencyTypesNot: ['type-only'] },
    },
    {
      name: 'lib-is-a-leaf',
      severity: 'error',
      comment:
        'src/lib holds the primitives every section draws alike, so it imports neither the ' +
        'shell nor a feature: a primitive that reached up into either would carry that ' +
        "part's concerns into every section that draws it.",
      from: { path: '^src/lib/' },
      to: { path: '^src/(shell|features)/' },
    },
    {
      name: 'not-to-unresolvable',
      severity: 'error',
      comment:
        'An import depcruise cannot resolve (a broken @/ alias, a missing file) matches no ' +
        'path rule above, so it would silently slip past every boundary.',
      from: {},
      to: { couldNotResolve: true },
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
