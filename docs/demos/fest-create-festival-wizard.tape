# fest create festival — multi-step Festival wizard (standard happy path)
#
# Fixture (set FEST_VHS_ROOT):
#   $FEST_VHS_ROOT/bin/fest
#   $FEST_VHS_ROOT/config/
#   $FEST_VHS_ROOT/workspace/              # fest init'd
#   $FEST_VHS_ROOT/workspace/.campaign/    # required: the project picker lists
#   $FEST_VHS_ROOT/workspace/projects/     # projects/* only inside a campaign
#
# Record from fest repo root:
#   FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-festival-wizard.tape
# Then copy out/fest-create-festival-wizard.gif → docs/demos/

Output out/fest-create-festival-wizard.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1100
Set Height 680
Set Padding 18
Set Margin 0
Set BorderRadius 0
Set Framerate 30
Set TypingSpeed 55ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#F2721C","magenta":"#C2410C","cyan":"#FFB347","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#FFB347","brightMagenta":"#EA5513","brightCyan":"#FFB347","brightWhite":"#FFFFFF"}

# Fixture must include config/config.json with tui.theme=dark (see fixtures/fest-create-tui/).
Hide
Type "unset NO_COLOR CI" Enter
Type "export PATH=$FEST_VHS_ROOT/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export CLICOLOR_FORCE=1" Enter
Type "export COLUMNS=100 LINES=36" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

# Jump straight into Festival wizard
Type "fest create festival" Sleep 400ms Enter
Sleep 1800ms

# Type: leave on standard* (default). Tab focuses the confirm, Enter submits.
Sleep 700ms
Tab
Sleep 300ms
Enter
Sleep 1200ms

# Identity: name and goal, then the separate Next/Back nav form.
Type "ecommerce-mvp"
Sleep 400ms
Tab
Sleep 300ms
Type "Launch MVP with catalog, cart, and checkout"
Sleep 500ms
Enter
Sleep 900ms
Enter
Sleep 1200ms

# Project: mode select defaults to "Pick from projects list"...
Tab
Sleep 300ms
Enter
Sleep 1200ms
# ...then choose demo-app from the list.
Tab
Sleep 300ms
Enter
Sleep 1200ms

# Seed: default is Skip, so Up selects "Paste a short brief".
Up
Sleep 300ms
Tab
Sleep 300ms
Enter
Sleep 1000ms
Type "Prior art: support tickets on cart drop-off"
Sleep 500ms
Tab
Sleep 300ms
Enter
Sleep 1200ms

# Tags
Type "backend,commerce"
Sleep 400ms
Tab
Sleep 300ms
Enter
Sleep 1400ms

# Confirm create
Sleep 800ms
Enter
Sleep 3500ms
