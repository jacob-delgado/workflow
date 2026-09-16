<!--
Thanks for contributing. The checklist below is the same gate CI runs, so
working through it locally is the fastest route to a green pull request.
-->

## What this changes

<!-- What the change does, and why. The why matters more: the diff already
     shows the what. -->

## Related issue

<!-- "Fixes #42", or why no issue was needed (typo fixes and obvious bugs can go
     straight to a pull request). -->

## Checklist

- [ ] `task check` passes locally — lint, tests, both coverage floors,
      `govulncheck`, `gitleaks`
- [ ] Tests cover the change. For a bug fix, a test that fails without the fix
      and passes with it
- [ ] Commits follow [Conventional Commits](https://www.conventionalcommits.org)
      (the `commit-msg` hook and CI both check this)
- [ ] New `.go` files carry the SPDX license header
- [ ] `task docs:gen` re-run and the result committed, if a command or flag
      changed
- [ ] No credential, token, or other secret appears in the diff, the tests, or
      the fixtures

<!-- Not sure about something? Open the pull request anyway and ask in it. An
     incomplete pull request with a question is more useful than a perfect one
     that never gets opened. -->
