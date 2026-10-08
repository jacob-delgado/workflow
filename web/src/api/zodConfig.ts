import { z } from 'zod'

// The server's content policy runs no script built from a string, which zod
// would otherwise try once, to compile its parsers, and the browser would
// report as a violation. Imported first, before any schema is made, so every
// one parses without.
z.config({ jitless: true })
