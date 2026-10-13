---
title: "Install"
weight: 10
---

# Install

## What it needs

- **git**, always.
- **[lefthook](https://lefthook.dev)**, for running hooks on demand and for
  turning existing `.git/hooks` into a `lefthook.yml`. Without it those two
  actions are not offered; commits still run whatever hooks git has.
- **`gh`**, optionally, as a place to find a GitHub token, and as the tool
  `forge.cli` routes GitHub calls through. See
  [Configuration]({{< relref "/docs/configuration" >}}).
- **`glab`**, only with `forge.cli` on GitLab, as the tool it routes GitLab
  calls through. Without it those calls go over HTTP. workflow does not read
  glab's own login: over HTTP it takes `$GITLAB_TOKEN`, `$GLAB_TOKEN` or
  `forge.token`, which needs the `api` scope to write.
- **[Taskwarrior](https://taskwarrior.org) 3.5.0 or newer**, optionally, for
  the Tasks pane and the `--web` Tasks section; without it they say so and
  nothing else changes. On macOS, `brew install task`. On Linux, your
  distribution's package where it carries 3.5.0 or newer — many still ship
  2.x, which is too old — or a build from
  [Taskwarrior's source](https://github.com/GothenburgBitFactory/taskwarrior),
  which needs Rust and CMake. Run Taskwarrior once, by its path where go-task
  comes first on `PATH`, so it writes its configuration. go-task, the Taskfile
  runner the source build below uses, is also called `task`; workflow tells
  the two apart, and `workflow doctor` names the Taskwarrior it found, or why
  none is usable.

`workflow doctor` reports which of these it finds, `gh` and `glab` both
whichever forge the repository is on, and `workflow doctor --online` warns
when a GitLab token can read but not write.

## With Go

The shortest path, if you have Go installed:

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@latest
```

The binary lands in `$(go env GOPATH)/bin`, which needs to be on your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

The binary carries the web interface (`workflow --web`) like a release binary
does; Go alone builds it, with no Node needed.

`@latest` resolves to the newest release tag. For a reproducible install, name
the version instead:

<!-- x-release-please-start-version -->

```sh
go install github.com/jacob-delgado/workflow/cmd/workflow@v0.9.0
```

<!-- x-release-please-end -->

## From a release

Each [release](https://github.com/jacob-delgado/workflow/releases/latest)
lists a binary for every platform it is built for, named `workflow_<os>_<arch>`
— with `.exe` on Windows — as `workflow_darwin_arm64` is for Apple Silicon
macOS. Download the one for your platform, along with `SHA256SUMS`, then verify
and install it:

```sh
sha256sum -c SHA256SUMS --ignore-missing
chmod +x workflow_darwin_arm64
mv workflow_darwin_arm64 /usr/local/bin/workflow
```

Every binary is built by GitHub Actions and carries a provenance attestation, so
you can confirm it came from this repository's CI rather than someone's laptop:

```sh
gh attestation verify workflow_darwin_arm64 --repo jacob-delgado/workflow
```

Each release also carries an SBOM, `workflow.spdx.json`, listing in SPDX format
the dependencies the binaries were built from — for checking them against
advisories or a policy.

## From source

```sh
git clone https://github.com/jacob-delgado/workflow.git
cd workflow
mise trust && mise install
task build            # builds bin/workflow
```

This needs [mise](https://mise.jdx.dev), which provisions the pinned toolchain —
Go, the linters, and everything else the build expects. See
[Development]({{< relref "/docs/contributing" >}}) for what else is in there.

## First run

```sh
workflow config init      # asks, then writes .workflow.json for you alone
workflow doctor           # says what is still missing
```

Set it up, answering the prompts, with `workflow config init` (`--template`
writes a blank file to edit) — [Configuration]({{< relref "/docs/configuration" >}})
explains where to get the tokens — and run `workflow` to open the TUI. Or run
`workflow`, or `workflow --web`, with no file at all: each offers to set one
up, asking the same questions.
[Using workflow]({{< relref "/docs/usage" >}}) walks through it.

## Shell completion

`workflow completion <shell>` prints a completion script for `bash`, `zsh`,
`fish` or `powershell`. Source it from your shell's startup, and `<tab>`
completes subcommands and flags — and `workflow branch <tab>` completes the keys
of the issues assigned to you.

```sh
# bash — for this shell now, or add to ~/.bashrc
source <(workflow completion bash)

# zsh — write it where your completions live
workflow completion zsh > "${fpath[1]}/_workflow"

# fish
workflow completion fish > ~/.config/fish/completions/workflow.fish
```

Run `workflow completion --help` for your shell's exact one-time setup.
