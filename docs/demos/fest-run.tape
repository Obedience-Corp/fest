# fest run — classify the next slice; optionally loop a caller-supplied worker
#
# Fest does not launch an agent. This tape uses a dummy ./worker that records
# the slice prompt from stdin. No Claude, Codex, or other harness.
#
# Build and record:
#   just build quick
#   FEST_VHS_ROOT="$(mktemp -d)" just vhs record docs/demos/fest-run.tape

Output out/fest-run.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 16
Set Width 1040
Set Height 620
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 24
Set TypingSpeed 55ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#4DA3FF","magenta":"#B388FF","cyan":"#66D9EF","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#60A5FA","brightMagenta":"#D8B4FE","brightCyan":"#67E8F9","brightWhite":"#FFFFFF"}

Hide
Type "unset NO_COLOR CI" Enter
Type "test -n $FEST_VHS_ROOT" Enter
Type "export PATH=$PWD/bin:$PATH" Enter
Type "bash docs/demos/fixtures/fest-run/setup.sh $FEST_VHS_ROOT" Enter
Type "export HOME=$FEST_VHS_ROOT/home" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

Sleep 400ms
Type "fest run" Sleep 300ms Enter
Sleep 2500ms

Type "fest run --exec ./worker --max-tasks 8 --max-minutes 5" Sleep 300ms Enter
Sleep 3500ms

Type "fest run --status" Sleep 300ms Enter
Sleep 2500ms
