# Fest explorer V3 — real dev binary, recorded against a disposable fixture.
#
# The fixture root is supplied by the recorder as FEST_VHS_ROOT. Keeping the
# path out of this tape makes the checked-in source safe to review and replay.
# The 14px font is intentional: the 1040px narrow-layout check showed that
# 18px clipped the explorer's split tree/preview layout at the planned width.
#
# Record with:
# FEST_VHS_ROOT=/path/to/fest-explorer-v3 just vhs record docs/demos/fest-explorer-v3.tape

Output out/fest-explorer-v3.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1040
Set Height 620
Set Padding 22
Set Margin 0
Set BorderRadius 0
Set Framerate 30
Set TypingSpeed 80ms
Set Theme {"name":"Obedience Corp","background":"#0D0D11","foreground":"#F8F8F2","selection":"#3A2418","cursor":"#F2721C","black":"#0D0D11","red":"#FF5F57","green":"#50FA7B","yellow":"#FEBC2E","blue":"#F2721C","magenta":"#C2410C","cyan":"#FFB347","white":"#F8F8F2","brightBlack":"#A1A1AA","brightRed":"#FF5F57","brightGreen":"#50FA7B","brightYellow":"#FEBC2E","brightBlue":"#FFB347","brightMagenta":"#EA5513","brightCyan":"#FFB347","brightWhite":"#FFFFFF"}

# Hidden fixture setup: the visible recording is produced only by the real
# fest binary and the real explorer TUI.
Hide
Type "unset NO_COLOR CI" Enter
Type "export PATH=$FEST_VHS_ROOT/bin:$PATH" Enter
Type "export FEST_CONFIG_DIR=$FEST_VHS_ROOT/config" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type "export PS1='\033[38;5;208m❯\033[0m '" Enter
Type "cd $FEST_VHS_ROOT/workspace" Enter
Type "clear" Enter
Show

# Canonical dark journey: status-filtered explorer → festival → phase preview.
Type "fest explore active" Sleep 600ms Enter
Sleep 3800ms
Enter Sleep 3200ms
Enter Sleep 3200ms
Type "j" Sleep 900ms
Enter Sleep 3500ms
Sleep 1800ms
Type "q"
Sleep 1000ms
