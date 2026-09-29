# fest watch: a deferred blocker is drawn exactly like a blocked one
#
# Build and record:
#   just build quick
#   FEST_VHS_ROOT="$(mktemp -d)" just vhs record docs/demos/fest-watch-deferral.tape

Output docs/demos/fest-watch-deferral.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1200
Set Height 800
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 24
Set TypingSpeed 40ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#4DA3FF","magenta":"#B388FF","cyan":"#66D9EF","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#60A5FA","brightMagenta":"#D8B4FE","brightCyan":"#67E8F9","brightWhite":"#FFFFFF"}

Hide
Type "unset NO_COLOR CI" Enter
Type "test -n $FEST_VHS_ROOT" Enter
Type "export PATH=$PWD/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export HOME=$FEST_VHS_ROOT/home && mkdir -p $HOME" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "export FEST_DEMO_REPO=$PWD" Enter
Type "FEST_DIR=$(bash $FEST_DEMO_REPO/docs/demos/fixtures/fest-watch-deferral/setup.sh $FEST_VHS_ROOT blocked)" Enter
Type "cd $FEST_DIR" Enter
Type "clear" Enter
Show

Sleep 1s
Type "fest watch" Sleep 400ms Enter
Sleep 4s
Ctrl+C
Sleep 1s

Hide
Type "clear" Enter
Type "bash $FEST_DEMO_REPO/docs/demos/fixtures/fest-watch-deferral/setup.sh $FEST_VHS_ROOT deferred >/dev/null" Enter
Type "clear" Enter
Show

Sleep 1s
Type "fest watch" Sleep 400ms Enter
Sleep 4s
Ctrl+C
Sleep 1500ms
