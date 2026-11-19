// The refusals the form holds, of a setting the server refused or of a row
// the form's own lists refuse, read out of react-hook-form's errors.

// A refusal as the form keeps one: what kind, and why.
interface Refusal {
  type?: unknown
  message?: unknown
}

// A refusal the form holds: the path of the setting refused, what kind of
// refusal it is, and why.
export interface HeldRefusal {
  path: string
  type: string
  message: string
}

// heldRefusals are every refusal the form holds, in the order it refused
// them: of a field, a list's row, or a list as a whole, whose path ends in
// root.
export function heldRefusals(errors: object, under = ''): HeldRefusal[] {
  return Object.entries(errors).flatMap(([key, held]: [string, unknown]) => {
    const path = under === '' ? key : `${under}.${key}`
    if (typeof held !== 'object' || held === null) {
      return []
    }

    const { type, message } = held as Refusal
    if (typeof message === 'string' && typeof type === 'string') {
      return [{ path, type, message }]
    }

    return heldRefusals(held, path)
  })
}

// refusedPaths are the paths of every setting the form holds refused, in the
// order it refused them.
export function refusedPaths(errors: object): string[] {
  return heldRefusals(errors).map(({ path }) => path)
}
