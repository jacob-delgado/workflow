# workflow

A terminal UI for the loop a developer actually runs all day: pick up a Jira
issue, start a branch for it, open the pull or merge request, and tell the team
in Slack — without leaving the keyboard or reconstructing the same context in
three browser tabs.

## Status

Early. The scaffolding is real and the gates are wired, but the integrations are
not built yet. What works today:

- `workflow` opens the TUI, which reports the configuration it found.
- `workflow config init` writes a starting configuration file.
- `workflow config show` prints the configuration in effect, tokens masked.
- `workflow doctor` says which file is in effect and what it is missing.

Jira, Slack, and the Git forge (GitHub or GitLab) are configured but not yet
called. Expect the configuration format to change while that lands.

## Install

With Go:

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@latest
```

The binary lands in `$(go env GOPATH)/bin`, which needs to be on your `PATH`.
For a reproducible install, name the version instead: `@v0.1.0`. Note that
`@latest` resolves to the newest release tag, and until the first tag exists, to
the most recent commit on `main`.

From a [release](https://github.com/jacob-delgado/workflow/releases) — binaries
are published for macOS, Linux, and Windows on amd64 and arm64, each with a
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
  "slack": {
    "token": "",
    "channel": "#dev-workflow"
  }
}
```

workflow reads `.workflow.json` from the current directory, and falls back to
your home directory. **A file in the current directory replaces the one in your
home directory** — they are never merged, so a repository-local configuration is
always the whole story.

### Jira token (on-premises / Data Center)

1. Sign in to your Jira instance in a browser.
2. Open your avatar menu → **Profile** → **Personal Access Tokens**.
3. Create a token and copy the value into `jira.token`, with your instance's URL
   in `jira.base_url`.

Leave `jira.user` empty to authenticate with that token as a bearer token, which
is what Data Center expects. Set `jira.user` only if your instance requires HTTP
Basic authentication, in which case the token is used as the password.

### Slack token

1. Create an app at <https://api.slack.com/apps> in your workspace.
2. Under **OAuth & Permissions**, add the `chat:write` bot token scope.
3. Install the app to the workspace and copy the **Bot User OAuth Token** — it
   starts with `xoxb-` — into `slack.token`.
4. Set `slack.channel` to the channel to post in, and invite the bot to it.

`workflow --help` repeats all of this at the terminal.

### Keeping the tokens safe

`.workflow.json` holds live credentials. `config init` writes it at mode `0600`,
it is listed in `.gitignore`, and `config show` masks both tokens. Nothing in
this repo will print a token in full.

## Development

```sh
task --list        # every task, with a description
task run           # run the TUI from source
task test          # tests with the race detector
task cover:branch  # condition coverage: which branches were never taken
task check         # the full gate: lint, coverage floors, vuln, secrets
```

`task check` is what CI runs. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
setup and conventions, and [CLAUDE.md](CLAUDE.md) for the code standards this
project holds itself to.

## Documentation

Full documentation is at
**[jacob-delgado.github.io/workflow](https://jacob-delgado.github.io/workflow/)**
— install, configuration, and a command reference generated from the code. The
source is in [`docs/`](docs/); `task docs:serve` previews it locally.

## Releases

Versioning is automated from the commit history with release-please: merging its
release pull request tags the version and publishes binaries for macOS, Linux,
and Windows (amd64 and arm64), each with a SHA256 checksum and a build
provenance attestation. There are no releases yet — the first one lands when the
integrations do.

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
