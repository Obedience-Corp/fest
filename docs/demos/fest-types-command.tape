# fest types command — discovery that matches create --type
#
# Fixture (set FEST_VHS_ROOT):
#   $FEST_VHS_ROOT/bin/fest          # branch binary
#   $FEST_VHS_ROOT/config/           # FEST_CONFIG_DIR (may be empty)
#   $FEST_VHS_ROOT/templates/        # nested methodology templates
#   $FEST_VHS_ROOT/workspace/        # cwd for the recording
#
# Record:
#   FEST_VHS_ROOT=/path/to/fixture vhs docs/demos/fest-types-command.tape

Output out/fest-types-command.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1100
Set Height 720
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 24
Set TypingSpeed 55ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#4DA3FF","magenta":"#B388FF","cyan":"#66D9EF","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#60A5FA","brightMagenta":"#D8B4FE","brightCyan":"#67E8F9","brightWhite":"#FFFFFF"}

Hide
Type "unset NO_COLOR CI" Enter
Type "export PATH=$FEST_VHS_ROOT/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export FEST_TEMPLATES_DIR=$FEST_VHS_ROOT/templates" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

# Unified list: festival workflow + phase packages + tasks
Type "fest types list" Sleep 400ms Enter
Sleep 3500ms

Type "clear" Enter
Sleep 400ms

# Phase detail for create phase --type
Type "fest types show implementation --level phase" Sleep 400ms Enter
Sleep 2800ms

Type "clear" Enter
Sleep 400ms

# Festival workflow type for create festival --type
Type "fest types show standard" Sleep 400ms Enter
Sleep 2800ms

Type "clear" Enter
Sleep 400ms

# Error path: no double 'not found', useful hint
Type "fest types show feature" Sleep 400ms Enter
Sleep 2200ms

Type "clear" Enter
Sleep 400ms

# Missing festival type points at list --level festival
Type "fest create festival --type quick --name demo 2>&1 | head -5" Sleep 400ms Enter
Sleep 2500ms
