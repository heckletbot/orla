# Demo Part 1: Customer support workflow - stage mapping only
#
# Types the snippet in vim. Tab for indent; noautoindent to avoid double-indent.
# Stage mapping: classify→light, policy/reply/route→heavy.
#
# Run from repo root: vhs vhs/demo_part1.tape
# Output: share/demo_part1.mp4

Output share/demo_part1.mp4

Require vim

Set Theme "3024 Night"
Set FontSize 22
Set Width 1200
Set Height 1200
Set LineHeight 1.3
Set TypingSpeed 30ms

# Open vim with empty .go file (noautoindent so Tab controls indent)
Hide
Type "rm -f /tmp/orla_demo_part1.go && vim -u NONE -c 'syntax on' -c 'set ft=go' -c 'set noautoindent' -c 'set indentexpr=' +startinsert /tmp/orla_demo_part1.go"
Enter
Show
Sleep 2s

# Type condensed stage mapping snippet
Type "// Part 1: Customer support workflow with Orla's stage mapping"
Enter
Type "// Stage mapping: classify -> light, policy/reply/route -> heavy"
Enter
Enter
Type "wf := orla.NewWorkflow(client)"
Enter
Enter
Type 'classifyStage := orla.NewStage("classify", lightBackend)'
Enter
Type 'policyStage := orla.NewStage("policy_check", heavyBackend)'
Enter
Type 'replyStage := orla.NewStage("reply", heavyBackend)'
Enter
Type 'routeStage := orla.NewStage("route_ticket", heavyBackend)'
Enter
Enter
Type "// Stage routing, reply branches on classify.needs_escalation"
Enter
Type 'replyStage.SetPromptBuilder('
Enter
Tab
Type 'func(r map[string]*orla.StageResult) (string, error) {'
Enter
Tab
Type 'var c struct { NeedsEscalation bool `json:"needs_escalation"` }'
Enter
Tab
Type "json.Unmarshal([]byte(r[classifyStage.ID].Response.Content), &c)"
Enter
Tab
Type 'if c.NeedsEscalation { return "ack...", nil }'
Enter
Tab
Type 'return "resolution...", nil'
Enter
Type "})"
Enter
Enter
Type "allStages := []*orla.Stage{classifyStage, policyStage, replyStage, routeStage}"
Enter
Enter
Type "// ..."
Enter
Enter
Type "wf.AddDependency(policyStage.ID, classifyStage.ID)"
Enter
Type "wf.AddDependency(replyStage.ID, policyStage.ID)"
Enter
Type "wf.AddDependency(routeStage.ID, classifyStage.ID)"
Enter
Enter
Type "wf.Execute(ctx)"
Enter

Escape
Sleep 10s
Type ":wq"
Enter
Sleep 1s
