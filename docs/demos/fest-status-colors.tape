# Fest status colors — show/watch readability review.
#
# Record with a disposable fixture containing the branch binary:
# FEST_VHS_ROOT=/path/to/fest-status-colors-fixture just vhs record docs/demos/fest-status-colors.tape

Output out/fest-status-colors.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1040
Set Height 700
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 30
Set TypingSpeed 80ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#4DA3FF","magenta":"#B388FF","cyan":"#66D9EF","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#60A5FA","brightMagenta":"#D8B4FE","brightCyan":"#67E8F9","brightWhite":"#FFFFFF"}

# Hidden fixture setup: the visible recording uses only the real fest binary.
Hide
Type "unset NO_COLOR CI" Enter
Type "export PATH=$FEST_VHS_ROOT/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "mkdir -p $FEST_VHS_ROOT/workspace/.campaign" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

# Show exposes the task hierarchy and in-progress highlight.
Type "fest show shared-brand-adoption --inprogress" Sleep 600ms Enter
Sleep 4500ms

# Watch uses the same tree renderer and keeps the output live.
Type "fest watch shared-brand-adoption" Sleep 600ms Enter
Sleep 3500ms
Ctrl+C
Sleep 1000ms
