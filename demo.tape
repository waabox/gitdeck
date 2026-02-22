# gitdeck demo recording
# Run with: vhs demo.tape

Output assets/demo.gif

Set Shell "zsh"
Set FontSize 14
Set Width 900
Set Height 550
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms
Set CursorBlink false

# Start gitdeck from repo
Hide
Type "cd /Users/waabox/code/waabox/gitdeck"
Enter
Sleep 500ms
Show

Type "gitdeck"
Sleep 300ms
Enter

# Let TUI load and show pipelines
Sleep 5s

# Navigate through pipelines
Down
Sleep 1200ms
Down
Sleep 1200ms

# Select a pipeline to see jobs
Enter
Sleep 2500ms

# Navigate jobs
Down
Sleep 800ms

# View logs
Type "l"
Sleep 3s

# Exit log viewer
Escape
Sleep 1500ms

# Quit
Type "q"
Sleep 500ms
