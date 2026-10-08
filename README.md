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
in Slack, Teams or Discord, or through a plain webhook — without leaving the
keyboard or reconstructing the same context in three browser tabs.

## Status

What works today, and what may still change before 1.0, is the
[Status](https://jacob-delgado.github.io/workflow/#status) on the
documentation site.

## Install

With Go:

<!-- x-release-please-start-version -->

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@v0.8.0
```

<!-- x-release-please-end -->

[Install](https://jacob-delgado.github.io/workflow/docs/install/) says what
workflow needs beside it, where that binary lands, and the other two ways to get
it: [from a release](https://jacob-delgado.github.io/workflow/docs/install/#from-a-release)
or [from source](https://jacob-delgado.github.io/workflow/docs/install/#from-source).
To build and check it in a container or a devcontainer instead, see
[Development setup](CONTRIBUTING.md#development-setup).

## Configure

[First run](https://jacob-delgado.github.io/workflow/docs/install/#first-run)
says how to set it up. `.workflow.json` looks like this:

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

workflow reads `.workflow.json` from the current directory or the nearest
directory above it, no higher than the repository root, and falls back to your
home directory. **A file found there is layered over the one in your home
directory**, setting by setting, so a repository's file holds only what that
repository changes — `{"jira": {"project": "OSS"}}`, say — and inherits the
rest, tokens included, without a copy of them. A token goes only where it was
written for: a repository's file that points Jira, the forge or the messaging
service somewhere else inherits none of your home file's credentials for it.

workflow keeps a little state between sessions on disk, never a secret;
[What is kept between sessions][kept] says what, where, and how to turn it off.

[kept]: https://jacob-delgado.github.io/workflow/docs/configuration/#what-is-kept-between-sessions

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
`discord` or `webhook`. Slack posts with a rotating user token or over an
incoming webhook; the others post over an incoming webhook, rendered in that
service's own markup.

**Slack, one transport or the other.** Set up a user token or a webhook; a file
that sets up both is refused.

**Incoming webhook**, the two-minute option: create an app at
<https://api.slack.com/apps>, turn on **Incoming Webhooks**, add one to the
workspace, pick its channel, and put the URL in `messaging.webhook_url`. It is
bound to that channel, so `messaging.channel` does not apply. Treat the URL as a
password.

**User token**, to post as you and choose the channel at runtime:

1. Create an app at <https://api.slack.com/apps> in your workspace.
2. Under **OAuth & Permissions**, add the `chat:write` user token scope and turn
   on token rotation. Tagging reviewers and user groups in an announcement is
   optional and also needs `users:read`, `channels:read`, `groups:read` and
   `usergroups:read`; without them the announcement posts untagged.
3. Install the app, and note the refresh token (`xoxe-1-…`) and the app's
   client ID and secret.
4. Set `messaging.channel` to a channel you are in, then run
   `workflow slack login`.

workflow refreshes the token before its twelve hours run out, and keeps it in the
macOS keychain, or in the configuration file on Linux and Windows.

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
`doctor` fails while anyone else can read it, `config init` warns when the file
is not ignored by git (add it to `.gitignore`), and `config show` masks every
credential — including `messaging.webhook_url`, which is a password that
happens to look like an address. Nothing in this repo will print a credential in
full.

## Use

Run `workflow` in a repository. Nine panes run down the left — Issues, Branch,
Commits, Review and your messaging service, named for it, in the order the work
goes, then Reviews, the pull requests waiting on your review, Tasks, your
Taskwarrior list, Summary, what you did yesterday or over any range of days,
and Repositories, where you work and the directories you keep as favorites —
and the one in focus fills the right. The bottom row shows
only the keys that do something right now.

| Key | Where | Does |
| --- | --- | --- |
| `tab` / `1`–`9` | anywhere | Move between panes |
| `t` / `c` / `b` | Issues | Change status, comment, branch for the issue |
| `space` / `a` / `c` | Commits | Stage a file, stage all, commit |
| `h` | Commits | Run the pre-commit hook |
| `P` | Branch | Push |
| `n` | Review | Open the pull or merge request |
| `p` | messaging | Preview the announcement; announce now or once CI passes |
| `?` | anywhere | Every key |

Commit bodies, pull request descriptions and announcements are written in
`$EDITOR`, comments in a vim-style box beside the issue, and each is previewed
before it sends. `workflow --dry-run` holds every
write back. The
[usage guide](https://jacob-delgado.github.io/workflow/docs/usage/) has the
whole loop.

`workflow --web` serves the loop in a browser instead, at
`http://127.0.0.1:13579`: nine sections — Issues, Branch, Review, your
messaging service, Reviews, Tasks, Summary, Repositories and Settings — kept
live by the server, in a light or a dark theme. `--web --dry-run` makes it
read-only. Only the offers to change a Taskwarrior task at the loop's moments
stay in the terminal, where the web has its Tasks buttons instead;
[the web page](https://jacob-delgado.github.io/workflow/docs/web/#what-stays-in-the-terminal)
says so.

## Development

[CONTRIBUTING.md](CONTRIBUTING.md) has the setup, the tasks and the gate a
change must pass; [ARCHITECTURE.md](ARCHITECTURE.md) how the system fits
together — the surfaces, the seams, and the two kinds of local state — and
[CLAUDE.md](CLAUDE.md) the code standards this project holds itself to.

## Documentation

Full documentation is at
**[jacob-delgado.github.io/workflow](https://jacob-delgado.github.io/workflow/)**
— install, usage, the web interface, configuration, and a command reference
generated from the code. The source is in [`docs/`](docs/); `task docs:serve`
previews it locally.

## Releases

Versioning is automated from the commit history with release-please.
[CONTRIBUTING.md](CONTRIBUTING.md#releases) says how a release is cut, and
[Install](https://jacob-delgado.github.io/workflow/docs/install/#from-a-release)
what each one carries.

## Security

Found a vulnerability? Please report it privately through a
[security advisory](https://github.com/jacob-delgado/workflow/security/advisories/new)
rather than opening an issue — see [SECURITY.md](SECURITY.md) for what to
include and what to expect.

## License

[Apache License 2.0](LICENSE). Contributions are accepted under the same terms;
see [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md).
