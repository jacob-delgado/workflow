// withWarning is what a post says once it is made, followed by the server's
// warning when it gave one: the announcement went, but the store could not
// confirm it, whose claim still keeps it from going again.
export function withWarning(said: string, warning: string | undefined): string {
  return warning === undefined || warning === '' ? said : `${said} ${warning}`
}
