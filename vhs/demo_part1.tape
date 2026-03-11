# Demo Part 1: Customer support workflow - stage mapping only
#
# Types the snippet in nvim. Uses Tab for indentation (avoids space/tab issues).
# Stage mapping: classify→light, policy/reply/route→heavy.
#
# Run from repo root: vhs vhs/demo_part1.tape
# Output: share/demo_part1.mp4

Output share/demo_part1.mp4

Require nvim

Set Theme "3024 Night"
Set FontSize 24
Set Width 1400
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 50ms

# Open nvim with empty .go file
Type "rm -f /tmp/orla_demo_part1.go && nvim -u NONE -c 'syntax on' -c 'set ft=go' +startinsert /tmp/orla_demo_part1.go"
Enter
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
Type 'replyStage.SetPromptBuilder(func(r map[string]*orla.StageResult) (string, error) {'
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
Sleep 1s
Type ":wq"
Enter
Sleep 1s
