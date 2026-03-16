# Demo Part 3: Part 2 + flush at boundary (two workflows)
#
# Starts from Part 2 snippet, edits in vim to add:
#   - wf.SetMemoryPolicy(orla.NewFlushAtBoundaryPolicy())
#   - Second workflow (wf2) - Orla flushes cache at boundary between them
#
# Run from repo root: vhs vhs/demo_part3.tape
# Output: share/demo_part3.mp4

Output share/demo_part3.mp4

Require vim

Set Theme "3024 Night"
Set FontSize 22
Set Width 1200
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 30ms

# Copy Part 2 snippet and open in vim
Hide
Type "cp vhs/demo_part2_snippet.go /tmp/orla_demo_part3.go && vim -u NONE -c 'syntax on' -c 'set ft=go' -c 'set noautoindent' -c 'set indentexpr=' /tmp/orla_demo_part3.go"
Enter
Show
Sleep 2s

# Edit line 1: Part 2 -> Part 3
Type "1G"
Type "cc"
Type "// Part 3: Part 2 + flush at boundary (second workflow gets clean cache)"
Escape
Sleep 0.5s

# Add SetMemoryPolicy after wf creation (line 4)
Type "4G"
Type "o"
Type "wf.SetMemoryPolicy(orla.NewFlushAtBoundaryPolicy())"
Escape
Sleep 0.5s

# Add second workflow after wf.Execute(ctx)
Type "32G"
Type "o"
Type "// Second workflow - Orla flushes cache at boundary"
Enter
Type "wf2 := orla.NewWorkflow(client)"
Enter
Type "wf2.SetMemoryPolicy(orla.NewFlushAtBoundaryPolicy())"
Enter
Type "wf2.Execute(ctx)"
Escape
Sleep 0.5s

Escape
Sleep 1s
Type ":wq"
Enter
Sleep 1s
