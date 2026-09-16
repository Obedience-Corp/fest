# Build: just build quick-stable
# Record: vhs docs/demos/fgo-picker-theme.tape
# No forced color: fgo captures stdout and the picker must detect stderr's TTY.
Output docs/demos/fgo-picker-theme.gif

Set Shell "zsh"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 17
Set Width 1200
Set Height 580
Set Padding 20
Set Framerate 20
Set TypingSpeed 80ms
Set Theme "Catppuccin Mocha"

Hide
Type "unset NO_COLOR CLICOLOR CLICOLOR_FORCE FORCE_COLOR CAMP_ROOT CI" Enter
Type "export TERM=xterm-256color COLORTERM=truecolor COLORFGBG=15\;0" Enter
Type `eval "$(FEST_BIN=$PWD/bin/fest bash docs/demos/fixtures/fgo-picker.sh)"` Enter
Sleep 1s
Type "cd ${FGO_FIXTURE:?fixture setup failed}/workspace || exit" Enter
Type "autoload -Uz compinit && compinit -D" Enter
Type `eval "$(command fest shell-init zsh)"` Enter
Type "export PS1='$ '" Enter
Type "clear" Enter
Show

Type "fgo FA" Enter
Sleep 2s
Type "002"
Sleep 1s
Tab
Sleep 1s
Down
Sleep 3s
Enter
Sleep 1s
Type "basename $PWD" Enter
Sleep 3s
