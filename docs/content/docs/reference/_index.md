---
title: "Command reference"
weight: 40
bookCollapseSection: true
---

# Command reference

Every command and flag, generated from the command tree itself by `task
docs:gen`, so what you read here is what the binary does. A check in the build
fails when the two disagree. The one command without a page is cobra's own
`help`: `workflow help COMMAND` prints the same text as `workflow COMMAND
--help`, and `workflow help` alone prints the root's.

The same text is available offline:

```sh
workflow --help
workflow config init --help
```

`workflow --help` is also where the token setup instructions live, for when the
website is not to hand.

What a script can rely on — the exit status of each kind of failure, which
stream carries what, and the JSON shapes — is on
[Scripting]({{< relref "/docs/scripting" >}}), beside each scriptable command's examples.
