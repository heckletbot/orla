# Demo Part 3: Type Orla API in nvim — Priority + Flush at boundary (two workflows)
#
# Run from repo root: vhs vhs/demo_part3.tape
# Output: share/demo_part3.mp4

Output share/demo_part3.mp4

Require nvim

Set Theme "3024 Night"
Set FontSize 24
Set Width 1400
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 50ms

# Open nvim with empty .go file (triggers syntax highlighting; temp path stays empty)
Type "rm -f /tmp/orla_demo_part3.go && nvim -u NONE -c 'syntax on' -c 'set ft=go' +startinsert /tmp/orla_demo_part3.go"
Enter
Sleep 2s

# Type the Orla API code (insert mode)
Type "// Orla API: Part 3 - Priority + Flush at boundary (two workflows)"
Enter
Enter
Type "wf := orla.NewWorkflow(client)"
Enter
Type "wf.SetMemoryPolicy(orla.NewFlushAtBoundaryPolicy())"
Enter
Enter
Type 'classifyStage := orla.NewStage("classify", lightBackend)'
Enter
Type "classifyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)"
Enter
Enter
Type 'policyStage := orla.NewStage("policy_check", heavyBackend)'
Enter
Type "policyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)"
Enter
Enter
Type 'replyStage := orla.NewStage("reply", heavyBackend)'
Enter
Type "replyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)"
Enter
Enter
Type 'routeStage := orla.NewStage("route_ticket", heavyBackend)'
Enter
Type "routeStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)"
Enter
Enter
Type "wf.Execute(ctx)"
Enter
Type "// Second workflow runs after - Orla flushes cache at boundary"
Enter
Type "wf2 := orla.NewWorkflow(client)"
Enter
Type "wf2.SetMemoryPolicy(orla.NewFlushAtBoundaryPolicy())"
Enter
Type "wf2.Execute(ctx)"
Enter

Escape
Sleep 1s
Type ":wq"
Enter
Sleep 1s
