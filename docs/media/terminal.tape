# The docs' recorded terminal: the whole loop in the terminal interface, over
# the demo's made-up world (internal/demo), from picking up PROJ-412 to its
# pull request announced in Slack. scripts/docs-media.sh plays it from the
# repository's root, once tmp/demo is built; `task docs:media` runs that.

Output docs/assets/images/terminal.gif

# Fixed, so a run on any machine films the same screen: the theme and the
# font by name (DejaVu Sans Mono draws every box and mark the interface uses),
# the time zone, and the size. 920 by 520 pixels at 14 pixels a character is
# a terminal of 96 columns by 30 rows (internal/demo's panes test draws the
# recorded diff, the proposed branch, the proposed reviewers and the opened
# pull request's notice at that size): roomier than 80 by 24, the smallest size the interface is
# designed for, which cuts the detail pane's lines short, and wide enough for
# the notice's whole line; and 920 pixels is about the width of the docs'
# column, so the GIF shows one pixel to a pixel there.
Set Shell "bash"
Set Theme "Catppuccin Mocha"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 14
Set Width 920
Set Height 520
Set Padding 12
Set TypingSpeed 35ms
Set CursorBlink false
# 25 frames a second, the rate the GIF plays at: VHS films in real time, and
# any frame it cannot take in time is time the GIF loses, so it asks for no
# more than the GIF shows. scripts/docs-media.sh fails a GIF that runs more
# than a second away from the tape's sleeps and typing, so this tape waits on
# nothing while shown.
Set Framerate 25
Env TZ "UTC"

# The demo starts out of frame, so the GIF opens on the interface, not on a
# shell.
Hide
Type "exec tmp/demo"
Enter
Sleep 1.5s
Show

# The issues assigned to Ana, PROJ-412 selected and in progress.
Sleep 2.5s

# Start work on it: workflow proposes the branch, and enter makes it.
Type "b"
Sleep 2s
Enter
Sleep 1.3s

# The changes and their diff, all staged, and committed with a subject the
# type and scope are already chosen for.
Type "3"
Sleep 1.2s
Type "a"
Sleep 1.8s
Type "c"
Sleep 0.6s
Type "redact tokens in the request log"
Sleep 0.6s
Enter
Sleep 1.3s

# Push the branch, once asked: the push's hook and the remote's answer for
# as long as the demo's push takes (cmd/demo's pushLatency), then the Branch
# pane with the branch pushed.
Type "2"
Sleep 0.6s
Type "P"
Sleep 1.8s
Enter
Sleep 2.8s

# Open the pull request: its base, then its title from the commit and its
# reviewers from CODEOWNERS.
Type "4"
Sleep 1.8s
Type "n"
Sleep 1.8s
Enter
Sleep 3s
Enter
Sleep 1.6s

# Slack waits for CI, then announces the pull request once it passes: the
# demo's checks pass four seconds after the pull request opens and the pane
# reads them every two (internal/demo), so this holds CI passed and the
# announcement on the Slack pane for about three seconds.
Type "5"
Sleep 7.5s

# Back to the Issues pane, the pull request announced and the footer saying
# so, held before the GIF loops back to the start on the same pane.
Type "1"
Sleep 3.5s
