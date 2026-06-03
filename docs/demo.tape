# Regenerate with:
#   env HOST=127.0.0.1 PORT=7681 vhs docs/demo.tape

Output docs/demo.gif

Set Shell "bash"
Set FontFamily "Menlo"
Set FontSize 20
Set LetterSpacing 0
Set LineHeight 1.15
Set Width 1200
Set Height 675
Set Padding 18
Set BorderRadius 12
Set WindowBar Colorful
Set Framerate 24
Set TypingSpeed 18ms

Hide
Type "go run ./cmd/mdpoke -- testdata/fixtures/comprehensive.md"
Enter
Sleep 3s
Show
Sleep 1s

Type "o"
Sleep 1s

Type "/"
Type "fixture"
Enter
Sleep 1s

Type "n"
Sleep 700ms

Type "?"
Sleep 1s

Type "copy"
Sleep 900ms

Escape
Sleep 500ms
Type "q"
