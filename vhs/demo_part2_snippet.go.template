// Part 2: Part 1 + priority scheduling (billing/technical get higher priority)
// Stage mapping: classify -> light, policy/reply/route -> heavy

wf := orla.NewWorkflow(client)

classifyStage := orla.NewStage("classify", lightBackend)
policyStage := orla.NewStage("policy_check", heavyBackend)
replyStage := orla.NewStage("reply", heavyBackend)
replyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)
routeStage := orla.NewStage("route_ticket", heavyBackend)

// Stage routing, reply branches on classify.needs_escalation
replyStage.SetPromptBuilder(
	func(r map[string]*orla.StageResult) (string, error) {
	var c struct { Category string `json:"category"` ; NeedsEscalation bool `json:"needs_escalation"` }
	json.Unmarshal([]byte(r[classifyStage.ID].Response.Content), &c)
	p := 5
	if c.Category == "billing" || c.Category == "technical" { p = 8 }
	replyStage.SetSchedulingHints(&orla.SchedulingHints{Priority: &p})
	if c.NeedsEscalation { return "ack...", nil }
	return "resolution...", nil
})

allStages := []*orla.Stage{classifyStage, policyStage, replyStage, routeStage}

// ...
wf.AddDependency(policyStage.ID, classifyStage.ID)
wf.AddDependency(replyStage.ID, policyStage.ID)
wf.AddDependency(routeStage.ID, classifyStage.ID)

wf.Execute(ctx)
