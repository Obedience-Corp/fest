# fest list --watch — event-driven lifecycle refresh without polling flashes
#
# Build and record:
#   just build quick
#   FEST_VHS_ROOT="$(mktemp -d)" just vhs record docs/demos/fest-list-watch-status.tape

Output docs/demos/fest-list-watch-status.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1100
Set Height 680
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 24
Set TypingSpeed 55ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#4DA3FF","magenta":"#B388FF","cyan":"#66D9EF","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#60A5FA","brightMagenta":"#D8B4FE","brightCyan":"#67E8F9","brightWhite":"#FFFFFF"}

Hide
Type "unset NO_COLOR CI" Enter
Type "test -n $FEST_VHS_ROOT" Enter
Type "bash docs/demos/fixtures/fest-list-watch-status/setup.sh $FEST_VHS_ROOT" Enter
Type "export PATH=$PWD/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "(sleep 5; mv festivals/active/launch-readiness-LW0001 festivals/ready/) &" Enter
Type "clear" Enter
Show

Type "fest list --watch" Sleep 600ms Enter
Sleep 8500ms
Ctrl+C
Sleep 1200ms
