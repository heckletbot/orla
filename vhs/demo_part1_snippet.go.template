// Part 1: Customer support workflow with Orla's stage mapping
// Stage mapping: classify -> light, policy/reply/route -> heavy

wf := orla.NewWorkflow(client)

classifyStage := orla.NewStage("classify", lightBackend)
policyStage := orla.NewStage("policy_check", heavyBackend)
replyStage := orla.NewStage("reply", heavyBackend)
routeStage := orla.NewStage("route_ticket", heavyBackend)

// Stage routing, reply branches on classify.needs_escalation
replyStage.SetPromptBuilder(
	func(r map[string]*orla.StageResult) (string, error) {
	var c struct { NeedsEscalation bool `json:"needs_escalation"` }
	json.Unmarshal([]byte(r[classifyStage.ID].Response.Content), &c)
	if c.NeedsEscalation { return "ack...", nil }
	return "resolution...", nil
})

allStages := []*orla.Stage{classifyStage, policyStage, replyStage, routeStage}

// ...
wf.AddDependency(policyStage.ID, classifyStage.ID)
wf.AddDependency(replyStage.ID, policyStage.ID)
wf.AddDependency(routeStage.ID, classifyStage.ID)

wf.Execute(ctx)
