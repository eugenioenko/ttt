# ttt hero demo. Record from the repo root:
#     make build && vhs docs-web/public/demo/demo.tape
# docs-web/demo/setup.sh builds a throwaway project in /tmp/ttt-demo and an
# isolated config in /tmp/ttt-demo-cfg, so recording touches nothing tracked.
# It opens straight on the code and ends on the welcome page, switching back
# to default-dark so the loop restarts in the same theme.

Output docs-web/public/demo/demo.webm
Output docs-web/public/demo/demo.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono, Symbols Nerd Font Mono"
Set FontSize 18
Set Width 1600
Set Height 900
Set Padding 0
Set Framerate 30
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type "bash docs-web/demo/setup.sh /tmp/ttt-demo /tmp/ttt-demo-cfg >/dev/null; TTT=$PWD/bin/ttt; cd /tmp/ttt-demo; clear; XDG_CONFIG_HOME=/tmp/ttt-demo-cfg/xdg TTT_CONFIG_DIR=/tmp/ttt-demo-cfg $TTT src/tasks.ts ."
Enter
Sleep 2.5s
Show

# 1. Rename every occurrence at once with multiple cursors
Sleep 1.2s
Ctrl+f
Type "task"
Sleep 300ms
Enter
Sleep 200ms
Escape
Sleep 300ms
Ctrl+k
Type "l"
Sleep 700ms
Type "entry"
Sleep 900ms
Escape
Sleep 300ms
Escape
Sleep 300ms
Ctrl+l
Type "w"

# 2. Live git: open the change as a side-by-side diff, drag the sidebar shut,
#    and keep coding in the diff
Ctrl+k
Type "c"
Sleep 800ms
Down
Enter
Sleep 600ms
Set TypingSpeed 30ms
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Alt+n
Set TypingSpeed 45ms
Sleep 500ms
Ctrl+g
Type@20ms "28"
Enter
Ctrl+l
Type "e"
Enter
Enter
Type "count(): number {"
Enter
Type "return this.pending().length;"
Enter
Type "}"
Sleep 1.4s
Ctrl+l
Type "w"
Sleep 300ms

# 3. Themes preview live while you type: nord, then Tokyo Neon
Ctrl+p
Type@25ms "Switch Theme"
Sleep 200ms
Enter
Sleep 300ms
Type@15ms "nord"
Sleep 1.3s
Backspace@15ms 4
Type@15ms "tokyo"
Sleep 600ms
Enter
Sleep 1s

# 4. Split the editor; the language server already offers the new method
Ctrl+l
Type "d"
Sleep 300ms
Set TypingSpeed 30ms
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Alt+w
Set TypingSpeed 45ms
Sleep 300ms
Ctrl+l
Type "s"
Sleep 300ms
Ctrl+k
Type "p"
Type@25ms "index"
Sleep 200ms
Enter
Sleep 500ms
Ctrl+g
Type@20ms "13"
Enter
Ctrl+l
Type "e"
Enter
Type "console.log(`${tracker."
Sleep 1.4s
Type@90ms "cou"
Sleep 500ms
Enter
Type "()} left`);"
Sleep 300ms
Ctrl+l
Type "w"
Sleep 1.2s

# 5. Integrated terminal, docked to the right
Ctrl+t
Sleep 600ms
Type "make test"
Sleep 200ms
Enter
Sleep 2.2s

# 6. A terminal in an editor tab, full width
Ctrl+t
Ctrl+l
Type "x"
Ctrl+b
Sleep 200ms
Ctrl+l
Type "t"
Sleep 700ms
Type@30ms "btop"
Enter
Sleep 3.2s
Type "q"
Sleep 400ms

# 7. Close everything, back to the welcome page and the default theme
Ctrl+l
Type "a"
Ctrl+l
Type "q"
Sleep 900ms
Ctrl+p
Type@25ms "Switch Theme"
Sleep 200ms
Enter
Sleep 300ms
Type@15ms "default-dark"
Sleep 400ms
Enter
Sleep 1.5s
