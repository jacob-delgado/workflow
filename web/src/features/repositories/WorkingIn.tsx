import type { Place } from '@/api/generated/types.gen.ts'

// WorkingIn is where the server works, drawn as the answer to "relative to
// what it changes": the repository it is in, then the path within it, muted,
// as one path; origin and the configuration that applies beneath.
export function WorkingIn({ place }: { place: Place }) {
  return (
    <section aria-labelledby="working-in" className="flex flex-col gap-item">
      <h2 id="working-in" className="text-lg font-semibold">
        Working in
      </h2>
      <p className="font-mono text-2xl wrap-anywhere">
        {place.root === '' ? (
          <span className="font-semibold">{place.shown}</span>
        ) : (
          <>
            <span className="font-semibold">{place.root_shown}</span>
            {place.within === '' ? null : (
              <span className="text-muted-foreground">/{place.within}</span>
            )}
          </>
        )}
      </p>
      <PlaceFacts place={place} />
    </section>
  )
}

// PlaceFacts are origin and the configuration of a place, in a list of
// terms, after saying it is in no repository where it is not: inside one,
// the path above already names it.
function PlaceFacts({ place }: { place: Place }) {
  return (
    <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-group gap-y-tight text-sm">
      {place.root === '' ? (
        <>
          <dt className="text-muted-foreground">Repository</dt>
          <dd>None: this directory is in no repository</dd>
        </>
      ) : null}
      <dt className="text-muted-foreground">Origin</dt>
      <dd className={place.origin === '' ? '' : 'font-mono'}>
        {place.origin === '' ? 'None' : place.origin}
      </dd>
      <dt className="text-muted-foreground">Configuration</dt>
      <dd>
        {place.config.length === 0 ? (
          'None, so the defaults apply'
        ) : (
          <ul className="flex flex-col">
            {place.config.map((file, index) => (
              <li key={file} className="font-mono">
                {file}
                {index === 0 && place.config.length > 1 ? (
                  <span className="font-sans text-muted-foreground"> over</span>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </dd>
    </dl>
  )
}
