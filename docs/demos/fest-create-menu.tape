# fest create — Create what? outer menu (Festival, Standalone workflow, Phase, Sequence, Task)
#
# Fixture (set FEST_VHS_ROOT):
#   $FEST_VHS_ROOT/bin/fest
#   $FEST_VHS_ROOT/config/
#   $FEST_VHS_ROOT/workspace/   # fest init'd cwd
#
# Record from fest repo root:
#   FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-create-menu.tape
# Then copy out/fest-create-menu.gif → docs/demos/fest-create-menu.gif

Output out/fest-create-menu.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1040
Set Height 480
Set Padding 18
Set Margin 0
Set BorderRadius 0
Set Framerate 30
Set TypingSpeed 70ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#F2721C","magenta":"#C2410C","cyan":"#FFB347","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#FFB347","brightMagenta":"#EA5513","brightCyan":"#FFB347","brightWhite":"#FFFFFF"}

Hide
Type "unset NO_COLOR CI" Enter
Type "export PATH=$FEST_VHS_ROOT/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

Type "fest create" Sleep 400ms Enter
Sleep 1800ms

# Walk peers including Standalone workflow
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Up
Sleep 400ms
Up
Sleep 400ms
Up
Sleep 400ms
Up
Sleep 700ms

# Open Standalone workflow stub, then back to menu
Down
Sleep 600ms
Enter
Sleep 2200ms

# Quit via Back (last option after return to menu)
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 250ms
Down
Sleep 500ms
Enter
Sleep 1000ms
