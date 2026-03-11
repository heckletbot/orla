# Demo Part 2: Type Orla API in nvim — Priority + Preserve (default policy)
#
# Run from repo root: vhs vhs/demo_part2.tape
# Output: share/demo_part2.mp4

Output share/demo_part2.mp4

Require nvim

Set Theme "3024 Night"
Set FontSize 24
Set Width 1400
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 50ms

# Open workflow_demo, navigate to reply stage priority scheduling
Type "nvim -u NONE -c 'syntax on' -c 'set ft=go' examples/workflow_demo/workflow_demo.go"
Enter
Sleep 2s

# Jump to reply stage SetSchedulingHints (priority from classify output)
Type ":516"
Enter
Sleep 1s
Type "zt"
Enter
Sleep 2s

Escape
Sleep 1s
Type ":q"
Enter
Sleep 1s
