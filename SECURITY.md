# Security Policy

## Supported versions

workflow is a small open-source project; only the **latest tagged release**
receives security updates. Older versions are not patched — upgrade to the
latest from
[GitHub Releases](https://github.com/jacob-delgado/workflow/releases).

| Version | Supported |
| --- | --- |
| Latest release | yes |
| Anything older | no — please upgrade first |

## Reporting a vulnerability

**Do not open a public GitHub issue for a security bug.** A public issue
discloses the problem before there is a fix, which puts every user at risk.

Report it privately instead:

> <https://github.com/jacob-delgado/workflow/security/advisories/new>

### What to include

The more of this you can provide, the faster the triage:

- **Version** affected, and any earlier versions you can confirm are affected.
- **Reproduction steps** — exact commands, configuration, or output.
- **Impact** — what an attacker can actually do, and a severity assessment if
  you have one.
- **Proof of concept**, if you have one. Minimal beats comprehensive.
- **Suggested fix**, if you have thought of one. Optional.

## What to expect

Honestly: **no guaranteed response time and no SLA.** This project is
maintained in spare time by one person. A first reply usually arrives within a
week or two; a fix and a release follow when there is a window of focused time.

If your situation is genuinely time-critical, the Apache 2.0 license exists for
exactly this — fork it and patch it yourself.

## Coordinated disclosure

Once a fix is ready:

1. The fix lands on `main` with a description that does not yet spell out the
   vulnerability in detail.
2. A patched release is cut.
3. The advisory is published, describing the issue and naming the reporter with
   their permission.

If you would rather remain anonymous, say so in the advisory and that is
honored.

## Scope

In scope:

- The workflow codebase: `cmd/`, `internal/`, `scripts/`, `build/`, and every
  file under `.github/workflows/`.
- The published release binaries and their checksums and attestations.
- Credential handling in particular — anything that writes a Jira or Slack
  token somewhere it should not go, logs one, or prints one unmasked, is a
  security bug. `.workflow.json` is written `0600`, is gitignored, and every
  code path that surfaces a token passes it through `config.Redact` first.
- Data at rest. The on-disk store (`internal/store`, a SQLite database under the
  OS-native data directory) keeps workflow state between sessions — today the
  commit scope last used per repository, and later what was announced and the
  last issue list seen. It **never** holds a secret: no token, no credential.
  The database is written `0600` inside a `0700` directory, so it is readable
  only by its owner. `store.disabled` turns it off entirely, keeping nothing on
  disk — anything the store persists that a token would not is still a bug.

Out of scope:

- Vulnerabilities in third-party dependencies. Report those upstream. This
  project tracks them with `govulncheck` in CI and Dependabot alerts, and bumps
  on the next dependency cycle — a disclosed CVE fix overrides the usual
  seven-day dependency age gate.
- Misconfiguration on your own host, such as a world-readable
  `.workflow.json` that you created by hand rather than with
  `workflow config init`.

## No telemetry

workflow makes **no analytics, crash-reporting, or phone-home network calls of
any kind.** It talks only to the services you configure — your Jira instance,
your Slack workspace, and your Git forge — and nowhere else.

If you find code that breaks that property, sending data anywhere the user did
not configure, that **is** a security bug. Report it the same way as any other.
