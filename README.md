# workflow

[![CI](https://github.com/jacob-delgado/workflow/actions/workflows/ci.yml/badge.svg)](https://github.com/jacob-delgado/workflow/actions/workflows/ci.yml)
[![CodeQL](https://github.com/jacob-delgado/workflow/actions/workflows/codeql.yml/badge.svg)](https://github.com/jacob-delgado/workflow/actions/workflows/codeql.yml)
[![Scorecard](https://github.com/jacob-delgado/workflow/actions/workflows/scorecard.yml/badge.svg)](https://github.com/jacob-delgado/workflow/actions/workflows/scorecard.yml)
[![Docs site](https://github.com/jacob-delgado/workflow/actions/workflows/pages.yml/badge.svg)](https://github.com/jacob-delgado/workflow/actions/workflows/pages.yml)
[![Release](https://github.com/jacob-delgado/workflow/actions/workflows/release.yml/badge.svg)](https://github.com/jacob-delgado/workflow/actions/workflows/release.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/jacob-delgado/workflow/badge)](https://scorecard.dev/viewer/?uri=github.com/jacob-delgado/workflow)
[![Latest release](https://img.shields.io/github/v/release/jacob-delgado/workflow?sort=semver)](https://github.com/jacob-delgado/workflow/releases/latest)
[![Go reference](https://pkg.go.dev/badge/github.com/jacob-delgado/workflow.svg)](https://pkg.go.dev/github.com/jacob-delgado/workflow)
[![Go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-FF75B7?logo=charm&logoColor=white)](https://github.com/charmbracelet/bubbletea)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-jacob--delgado.github.io%2Fworkflow-7B36ED?logo=gitbook&logoColor=white)](https://jacob-delgado.github.io/workflow/)
[![Conventional Commits](https://img.shields.io/badge/commits-Conventional-fe5196?logo=conventionalcommits&logoColor=white)](https://www.conventionalcommits.org/en/v1.0.0/)
[![Code of Conduct](https://img.shields.io/badge/code%20of%20conduct-Contributor%20Covenant-purple)](CODE_OF_CONDUCT.md)
[![Security policy](https://img.shields.io/badge/security-policy-critical)](SECURITY.md)

A terminal UI for the loop a developer actually runs all day: pick up a Jira
issue, start a branch for it, open the pull or merge request, and tell the team
in Slack — without leaving the keyboard or reconstructing the same context in
three browser tabs.

## Status

The whole loop works, and is new: expect rough edges, and a configuration
format that may still change before 1.0.

- `workflow` opens the TUI. Pick up an assigned Jira issue, comment on it or
  change its status, branch for it, stage and commit through the repository's
  own hooks — opening a failure at its line in `$EDITOR` — push, open the pull
  or merge request from the repository's template, follow its CI, and announce
  it in Slack. `?` lists the keys.
- `workflow --dry-run` does all of that with every write held back, saying what
  it would have done.
- With no Jira configured, the Issues pane lists the issues assigned to you on
  your forge (GitHub or GitLab) instead — pick one up, branch for it, and the
  pull request closes it on merge.
- A repository with hooks in `.git/hooks` and no lefthook configuration is
  offered a `lefthook.yml` that runs them.
- `workflow doctor` reports the repository, tooling and configuration in effect;
  `workflow doctor --online` asks Jira, Slack and your forge whether each
  credential actually works.
- `workflow config init` writes a starting configuration file, and
  `workflow config show` prints the one in effect, credentials masked.
- `workflow standup` drafts what you did — your recent commits, the issues you
  touched and the open pull requests on your branches — for you to edit and,
  optionally, post to Slack.
- `workflow reviews` lists the pull requests on your forge that are waiting on
  your review — the longest-waiting first, with the author, how CI stands and
  how long each has waited.

## Install

With Go:

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@latest
```

The binary lands in `$(go env GOPATH)/bin`, which needs to be on your `PATH`.
For a reproducible install, name the version instead: `@v0.0.5`. Note that
`@latest` resolves to the newest release tag, and until the first tag exists, to
the most recent commit on `main`.

From a [release](https://github.com/jacob-delgado/workflow/releases) — binaries
are published for macOS (arm64), Linux (amd64) and Windows (amd64), each with a
checksum and a build provenance attestation:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify workflow_darwin_arm64 --repo jacob-delgado/workflow
```

From source, using [mise](https://mise.jdx.dev) for the pinned toolchain and
[go-task](https://taskfile.dev) as the runner:

```sh
git clone https://github.com/jacob-delgado/workflow.git
cd workflow
mise trust && mise install
task build            # builds bin/workflow
```

There is also a devcontainer (VS Code, GoLand, Codespaces, or the devcontainer
CLI), and a build container for running the same checks without installing
anything: `task container:check`.

## Configure

```sh
workflow config init      # writes .workflow.json here, readable only by you
workflow doctor           # says what is still missing
```

`.workflow.json` looks like this:

```json
{
  "jira": {
    "base_url": "https://jira.example.com",
    "token": "",
    "user": ""
  },
  "messaging": {
    "kind": "slack",
    "token": "",
    "webhook_url": "",
    "channel": "#dev-workflow"
  },
  "forge": {
    "kind": "",
    "host": "",
    "token": ""
  },
  "ui": {
    "mouse": true,
    "ascii": false
  }
}
```

workflow reads `.workflow.json` from the current directory, and falls back to
your home directory. **A file in the current directory replaces the one in your
home directory** — they are never merged, so a repository-local configuration is
always the whole story.

workflow keeps a little state between sessions in an on-disk store — the commit
scope you last used, which pull requests you have announced, and the last issue
list it saw — under your platform's data directory, and never a secret. It is on
by default; set `"store": { "disabled": true }` to keep nothing on disk. See
[Configuration](https://jacob-delgado.github.io/workflow/docs/configuration/) for
where it lives and how it is keyed.

### Jira token (on-premises / Data Center)

1. Sign in to your Jira instance in a browser.
2. Open your avatar menu → **Profile** → **Personal Access Tokens**.
3. Create a token and copy the value into `jira.token`, with your instance's URL
   in `jira.base_url`.

Leave `jira.user` empty to authenticate with that token as a bearer token, which
is what Data Center expects. Set `jira.user` only if your instance requires HTTP
Basic authentication, in which case the token is used as the password.

**Behind single sign-on (Azure AD / Office 365)?** SSO usually guards only the
web UI, so a personal access token still reaches the REST API directly — no
special handling needed. When a gateway sits in front of the API too, the
[configuration guide][sso] shows how to tell (a one-line `curl`), and how to
carry the gateway's own token or headers with `jira.token_command` and
`jira.headers`.

[sso]: https://jacob-delgado.github.io/workflow/docs/configuration/

### Messaging: Slack, Teams, Discord or a plain webhook

`messaging.kind` picks the service — `slack` (the default when empty), `teams`,
`discord` or `webhook`. Slack posts over a bot token or an incoming webhook; the
others post over an incoming webhook, rendered in that service's own markup.

**Slack, either transport.** Set a webhook or a bot token; if you set both, the
bot token wins.

**Incoming webhook**, the two-minute option: create an app at
<https://api.slack.com/apps>, turn on **Incoming Webhooks**, add one to the
workspace, pick its channel, and put the URL in `messaging.webhook_url`. It is
bound to that channel, so `messaging.channel` does not apply. Treat the URL as a
password.

**Bot token**, to choose the channel at runtime:

1. Create an app at <https://api.slack.com/apps> in your workspace.
2. Under **OAuth & Permissions**, add the `chat:write` bot token scope.
3. Install the app to the workspace and copy the **Bot User OAuth Token** — it
   starts with `xoxb-` — into `messaging.token`.
4. Set `messaging.channel` to the channel to post in, and invite the bot to it.

**Teams, Discord or a plain webhook**: create an incoming webhook in the service,
set `messaging.kind` accordingly, and put the URL in `messaging.webhook_url`.

A configuration written before this block was renamed still names it `slack`;
rename the key to `messaging` and add `"kind": "slack"`. `workflow --help`
repeats all of this at the terminal.

### Reaching a forge behind SSO

If a bare token cannot reach your forge — an SSO gateway in front of it, say —
set `forge.cli` to `true`. workflow then routes its GitHub or GitLab API calls
through `gh` or `glab`, reusing the login those tools already hold, and falls
back to HTTP when the tool is not installed.

### Keeping the tokens safe

`.workflow.json` holds live credentials. `config init` writes it at mode `0600`,
`doctor` fails while anyone else can read it, it is listed in `.gitignore`, and `config show` masks every credential —
including `messaging.webhook_url`, which is a password that happens to look like
an address. Nothing in this repo will print a credential in full.

## Use

Run `workflow` in a repository. Five panes run down the left — Issues, Branch,
Commits, Review, Slack — in the order the work goes, and the one in focus fills
the right. The bottom row shows only the keys that do something right now.

| Key | Where | Does |
| --- | --- | --- |
| `tab` / `1`–`5` | anywhere | Move between panes |
| `t` / `c` / `b` | Issues | Change status, comment, branch for the issue |
| `space` / `a` / `c` | Commits | Stage a file, stage all, commit |
| `h` | Commits | Run the pre-commit hook |
| `P` | Branch | Push |
| `n` | Review | Open the pull or merge request |
| `p` | Slack | Preview the announcement; post now or once CI passes |
| `?` | anywhere | Every key |

Comments, commit bodies, pull request descriptions and Slack posts are written
in `$EDITOR` and previewed before they send. `workflow --dry-run` holds every
write back. The
[usage guide](https://jacob-delgado.github.io/workflow/docs/usage/) has the
whole loop.

## Development

```sh
task --list        # every task, with a description
task run           # run the TUI from source
task test          # tests with the race detector
task cover:branch  # condition coverage: which branches were never taken
task check         # the full gate: lint, tests, coverage floors, vuln, secrets
```

`task check` is what CI runs. See [ARCHITECTURE.md](ARCHITECTURE.md) for how the
system fits together — the surfaces, the seams, and the two kinds of local state
— [CONTRIBUTING.md](CONTRIBUTING.md) for the setup and conventions, and
[CLAUDE.md](CLAUDE.md) for the code standards this project holds itself to.

## Documentation

Full documentation is at
**[jacob-delgado.github.io/workflow](https://jacob-delgado.github.io/workflow/)**
— install, usage, configuration, and a command reference generated from the
code. The source is in [`docs/`](docs/); `task docs:serve` previews it locally.

## Releases

Versioning is automated from the commit history with release-please: merging its
release pull request tags the version and publishes binaries for macOS (arm64),
Linux (amd64) and Windows (amd64), each with a SHA256 checksum and a build
provenance attestation. There are no releases yet.

## Security

Found a vulnerability? Please report it privately through a
[security advisory](https://github.com/jacob-delgado/workflow/security/advisories/new)
rather than opening an issue — see [SECURITY.md](SECURITY.md) for what to
include and what to expect.

`.workflow.json` holds live credentials. It is written `0600`, gitignored, and
every path that surfaces a token masks it first.

## License

[Apache License 2.0](LICENSE). Contributions are accepted under the same terms;
see [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md).
