# The docs' recorded terminal: the whole loop in the terminal interface, over
# the demo's made-up world (internal/demo), from picking up PROJ-412 to its
# pull request announced in Slack. scripts/docs-media.sh plays it from the
# repository's root, once tmp/demo is built; `task docs:media` runs that.

Output docs/assets/images/terminal.gif

# Fixed, so a run on any machine films the same screen: the theme and the
# font by name (DejaVu Sans Mono draws every box and mark the interface uses),
# the time zone, and the size. 900 by 520 pixels at 14 pixels a character is
# a terminal of about 104 columns by 29 rows: wide enough that the rail and
# the detail pane both read whole, where 80 by 24, the smallest size the
# interface is designed for, cuts the titles short; and 900 pixels is about
# the width of the docs' column, so the GIF shows one pixel to a pixel there.
Set Shell "bash"
Set Theme "Catppuccin Mocha"
Set FontFamily "DejaVu Sans Mono"
Set FontSize 14
Set Width 900
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
Sleep 1.6s

# Start work on it: workflow proposes the branch, and enter makes it.
Type "b"
Sleep 0.8s
Enter
Sleep 1.3s

# The changes and their diff, all staged, and committed with a subject the
# type and scope are already chosen for.
Type "3"
Sleep 1.2s
Type "a"
Sleep 0.8s
Type "c"
Sleep 0.6s
Type "redact tokens in the request log"
Sleep 0.6s
Enter
Sleep 1.3s

# Push the branch, once asked.
Type "2"
Sleep 0.6s
Type "P"
Sleep 0.9s
Enter
Sleep 1.2s

# Open the pull request: its base, then its title from the commit.
Type "4"
Sleep 0.6s
Type "n"
Sleep 1s
Enter
Sleep 0.9s
Enter
Sleep 1.6s

# Slack waits for CI, then announces the pull request once it passes.
Type "5"
Sleep 4.6s
