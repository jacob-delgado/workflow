---
title: "Development"
weight: 30
---

# Development

How to contribute is written down once, in
[CONTRIBUTING.md](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md),
and the code standards in
[CLAUDE.md](https://github.com/jacob-delgado/workflow/blob/main/CLAUDE.md):

- [Development setup](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#development-setup)
  — mise and the pinned toolchain, the git hooks, and the containers that run
  the gate with nothing installed.
- [Day to day](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#day-to-day)
  — the tasks, and `task check`, the gate, part by part.
- [Tests](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#tests)
  — black-box, Arrange-Act-Assert, and the coverage floors.
- [Commit messages](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#commit-messages)
  and [Releases](https://github.com/jacob-delgado/workflow/blob/main/CONTRIBUTING.md#releases)
  — Conventional Commits, and how they cut a release.

This page covers the one thing those leave to it: this site.

## Documentation

This site is Hugo, built from Markdown in `docs/`:

```sh
task docs:serve    # preview at http://localhost:1313
task docs:build    # what CI builds and publishes
```

The command reference is **generated from the Cobra command tree** by `task
docs:gen`. Do not edit those pages by hand — `task docs:check` fails when they
disagree with the code, which is what keeps a new flag from shipping
undocumented.

### Images

The pictures on these pages, the recording of the terminal and the
screenshots of the web page, are files in `docs/assets/images`, shown with
the theme's `image` shortcode, and are made, never drawn by hand. When a
screen they show changes, make them again on Linux:

```sh
task docs:media    # every image in docs/assets/images, written again
```

It builds the demo, a made-up day's work the terminal runs over
(`cmd/demo`), and plays `docs/media/terminal.tape` over it with
[VHS](https://github.com/charmbracelet/vhs) into `terminal.gif`, and has
Playwright screenshot each section the web page describes, served over the
same demo by `cmd/demo -web`. VHS comes from `mise install`, or is built into `tmp/` where mise
cannot install it. ttyd is checked against the SHA-256 that
`docs/media/tool-checksums.txt` pins before the task runs it, since mise
installs it unchecked: the one `mise install` puts on your PATH is used when
it matches, the task stops on one that does not, and with none it downloads
ttyd into `tmp/` and checks that. Chromium, which films the terminal, and
ffmpeg, which encodes it, must already be on your machine: the task films
with the Chromium `PLAYWRIGHT_CHROMIUM_EXECUTABLE` names, or else the
`chromium` on your PATH, and refuses to run without one. It fails when an
image is past its budget, 300 KiB a screenshot and 2 MiB the recording.

Commit the change the images are to show first: the task refuses to run
while the demo, the interface, the tape or the web page has changes git has
not committed, so the images committed are of the code committed with them.
Run it on an idle machine, too. VHS films in real time, and on a machine busy
with a build it misses frames, so the recording skips past what the tape
holds on screen; the task fails when the recording runs more than a second
longer or shorter than the tape's sleeps and typing take, and a tape that
waits on the screen, which has no such length, is refused.

Look at each image before you commit it. Nothing else will: no gate or CI
job runs the task or compares the images, since fonts, Chromium and ffmpeg
change their bytes from one machine to the next.
