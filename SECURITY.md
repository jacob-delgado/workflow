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

- The workflow codebase: `cmd/`, `internal/`, `api/`, `web/`, `scripts/`,
  `build/`, and every file under `.github/workflows/`.
- The `workflow --web` surface: the local server it binds to `127.0.0.1` (the
  REST API `api/openapi.yaml` describes, and its event stream), and the
  browser app in `web/` that the release binary embeds and serves.
- The published release binaries and their checksums and attestations.
- Credential handling in particular — anything that writes a Jira, forge or
  messaging credential (a token, or a webhook URL, which is itself the
  credential) somewhere it should not go, logs one, or prints one unmasked, is
  a security bug. `.workflow.json` is written `0600`, `workflow config init`
  and the first run warn when git would not ignore it, and every code path
  that surfaces a token passes it through `config.Redact` first.
- The configuration's trust boundary. A `.workflow.json` in a repository, or in
  the current directory outside one, may have come from anyone, so it may not
  set what runs a program or reads the environment (`jira.token_command`,
  `jira.token_env`, `taskwarrior.program`) and inherits no home credential for
  an address it moves; only `~/.workflow.json` may. No configuration file is
  read that another user owns or that others may write. A way around any of
  these is a security bug.
- Data at rest. The on-disk store (`internal/store`) keeps workflow state
  between sessions in two SQLite files under the OS-native data directory.
  `workflow.db` is a cache of conveniences a session can make again: the
  commit scope last used per repository, what was announced there, and the
  last issue list seen (the non-secret fields a first pane needs — issue keys,
  summaries, statuses, status categories, types and priorities — so a session
  can open on it before the tracker answers). `kept.db` holds what the user
  decided: which Slack user or user group each forge owner is, with the label
  each was last seen with, or that an owner is not on Slack; the user groups
  each repository offers and the ones chosen last; the Slack workspace ID each
  of those decisions belongs to; and the paths of the directories marked as
  favorites. Neither **ever** holds a secret: no token, no credential. A
  repository is keyed by its remote reduced to the credential-free host and
  path, or by its root path when there is no remote it can parse; an owner by
  the forge's host and the owner's name; the Jira instance by a hash of its
  URL, and the cached issue list by that hash and the list's JQL query; and a
  favorite by its path. Where the filesystem keeps Unix modes, the store's
  directory is made `0700` and each file `0600`, which also keeps the `-wal`
  and `-shm` files SQLite writes beside them from anyone else, so both are
  readable only by their owner. `store.disabled` turns the store off entirely,
  keeping nothing on disk. A credential reaching either file is a bug.

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
your Git forge and your messaging service (Slack, Teams, Discord or a webhook
you name) — and nowhere else.

If you find code that breaks that property, sending data anywhere the user did
not configure, that **is** a security bug. Report it the same way as any other.
