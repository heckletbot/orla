# Demo Part 2: Part 1 + minimal priority scheduling
#
# Starts from Part 1 snippet, edits in vim to add:
#   - replyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)
#   - Category in struct + SetSchedulingHints based on classify output
#
# Run from repo root: vhs vhs/demo_part2.tape
# Output: share/demo_part2.mp4

Output share/demo_part2.mp4

Require vim

Set Theme "3024 Night"
Set FontSize 22
Set Width 1200
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 30ms

# Copy Part 1 snippet and open in vim
Hide
Type "cp vhs/demo_part1_snippet.go /tmp/orla_demo_part2.go && vim -u NONE -c 'syntax on' -c 'set ft=go' -c 'set noautoindent' -c 'set indentexpr=' /tmp/orla_demo_part2.go"
Enter
Show
Sleep 2s

# Edit line 1: Part 1 -> Part 2
Type "1G"
Enter
Type "cc"
Enter
Type "// Part 2: Part 1 + priority scheduling (billing/technical get higher priority)"
Enter
Escape
Sleep 0.5s

# Add SetSchedulingPolicy after replyStage creation (line 9)
Type "9G"
Enter
Type "o"
Enter
Type "replyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)"
Enter
Escape
Sleep 0.5s

# Edit struct: add Category for priority logic
Type "15G"
Enter
Type "cc"
Enter
Type 'var c struct { Category string `json:"category"` ; NeedsEscalation bool `json:"needs_escalation"` }'
Enter
Escape
Sleep 0.5s

# Add priority hints before if c.NeedsEscalation (insert above)
Type "17G"
Enter
Type "O"
Enter
Tab
Type "p := 5"
Enter
Tab
Type 'if c.Category == "billing" || c.Category == "technical" { p = 8 }'
Enter
Tab
Type "replyStage.SetSchedulingHints(&orla.SchedulingHints{Priority: &p})"
Enter
Escape
Sleep 0.5s

Escape
Sleep 1s
Type ":wq"
Enter
Sleep 1s
