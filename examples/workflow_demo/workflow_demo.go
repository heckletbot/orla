// Package workflowdemo demonstrates the full Orla abstraction stack:
// Workflow -> Agent -> Stage DAG -> StageMapping -> Scheduling -> Context Passing.
//
// Pipeline: a customer support ticket triage and resolution workflow.
//
//	Workflow
//	  |
//	  +-- Agent "triage" (light model, FCFS)
//	  |     +-- Stage "classify"   (category + key issue, structured output)
//	  |     +-- Stage "sentiment"  (sentiment + urgency signals, structured output)
//	  |     |   (classify and sentiment run in parallel — no dependency between them)
//	  |     +-- Stage "prioritize" (severity + priority score; depends on both classify and sentiment)
//	  |
//	  +-- Agent "resolver" (heavy model, Priority scheduling; depends on "triage")
//	  |     +-- Stage "draft_response" (personalized reply using triage context)
//	  |     +-- Stage "policy_check"   (identify applicable policies using triage context)
//	  |     |   (draft_response and policy_check run in parallel)
//	  |     +-- Stage "final_review"   (review draft against policies; depends on both)
//	  |
//	  +-- Agent "escalation" (light model, FCFS; depends on "triage", parallel with "resolver")
//	        +-- Stage "route_ticket"   (escalation decision, structured output)
package workflowdemo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	orla "github.com/dorcha-inc/orla/pkg/api"
)

const (
	defaultOrlaURL    = "http://localhost:8081"
	defaultLightURL   = "http://sglang-light:30000/v1"
	defaultHeavyURL   = "http://sglang:30000/v1"
	defaultLightModel = "Qwen/Qwen3-4B-Instruct-2507"
	defaultHeavyModel = "Qwen/Qwen3-8B"
)

var classifySchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"category": map[string]any{
			"type": "string",
			"enum": []any{"billing", "technical", "account", "shipping", "general"},
		},
		"product": map[string]any{
			"type":        "string",
			"description": "Product or service mentioned in the ticket",
		},
		"key_issue": map[string]any{
			"type":        "string",
			"description": "One-sentence summary of the core issue",
		},
	},
	"required": []any{"category", "product", "key_issue"},
}

var sentimentSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"sentiment": map[string]any{
			"type": "string",
			"enum": []any{"frustrated", "neutral", "positive"},
		},
		"urgency_signals": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Phrases or cues from the ticket that indicate urgency",
		},
	},
	"required": []any{"sentiment", "urgency_signals"},
}

var prioritySchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"severity": map[string]any{
			"type": "string",
			"enum": []any{"critical", "high", "medium", "low"},
		},
		"priority": map[string]any{
			"type":        "integer",
			"description": "Scheduling priority from 1 (lowest) to 10 (highest)",
		},
		"reasoning": map[string]any{
			"type":        "string",
			"description": "One-sentence explanation of the severity assignment",
		},
	},
	"required": []any{"severity", "priority", "reasoning"},
}

var escalationSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"escalate": map[string]any{
			"type":        "boolean",
			"description": "Whether this ticket requires human escalation",
		},
		"reason": map[string]any{
			"type":        "string",
			"description": "One-sentence explanation of the escalation decision",
		},
		"suggested_team": map[string]any{
			"type": "string",
			"enum": []any{"billing", "technical", "account_management", "none"},
		},
	},
	"required": []any{"escalate", "reason", "suggested_team"},
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Run executes the customer support workflow demo.
// ticket is the raw customer support message.
func Run(ctx context.Context, ticket string) error {
	orlaURL := envOr("ORLA_URL", defaultOrlaURL)
	client := orla.NewOrlaClient(orlaURL)
	if err := client.Health(ctx); err != nil {
		return fmt.Errorf("orla health check: %w", err)
	}

	// --- Backends ---
	var lightBackend, heavyBackend *orla.LLMBackend
	if vllmLight := os.Getenv("VLLM_LIGHT_URL"); vllmLight != "" && os.Getenv("VLLM_HEAVY_URL") != "" {
		vllmHeavy := os.Getenv("VLLM_HEAVY_URL")
		lightBackend = orla.NewVLLMBackend(envOr("LIGHT_MODEL", defaultLightModel), vllmLight)
		heavyBackend = orla.NewVLLMBackend(envOr("HEAVY_MODEL", defaultHeavyModel), vllmHeavy)
	} else {
		lightBackend = orla.NewSGLangBackend(
			envOr("LIGHT_MODEL", defaultLightModel),
			envOr("SGLANG_LIGHT_URL", defaultLightURL),
		)
		heavyBackend = orla.NewSGLangBackend(
			envOr("HEAVY_MODEL", defaultHeavyModel),
			envOr("SGLANG_HEAVY_URL", defaultHeavyURL),
		)
	}
	if err := client.RegisterBackend(ctx, lightBackend); err != nil {
		return fmt.Errorf("register light backend: %w", err)
	}
	if err := client.RegisterBackend(ctx, heavyBackend); err != nil {
		return fmt.Errorf("register heavy backend: %w", err)
	}

	// --- Agent 1: triage (light model, three-stage diamond DAG) ---
	//
	//   classify ──┐
	//              ├──▶ prioritize
	//   sentiment ─┘
	//
	triage := orla.NewAgent(client)
	triage.Name = "triage"

	classifyStage := orla.NewStage("classify", lightBackend)
	classifyStage.SetMaxTokens(512)
	classifyStage.SetTemperature(0)
	classifyStage.SetSchedulingPolicy(orla.SchedulingPolicyFCFS)
	classifyStage.SetResponseFormat(orla.NewStructuredOutputRequest("ticket_classify", classifySchema))
	classifyStage.Prompt = fmt.Sprintf(
		"You are a customer support triage system. Classify this support ticket: extract the category, product, and a one-sentence summary of the core issue.\n\nTicket:\n%s", ticket)

	sentimentStage := orla.NewStage("sentiment", lightBackend)
	sentimentStage.SetMaxTokens(256)
	sentimentStage.SetTemperature(0)
	sentimentStage.SetSchedulingPolicy(orla.SchedulingPolicyFCFS)
	sentimentStage.SetResponseFormat(orla.NewStructuredOutputRequest("ticket_sentiment", sentimentSchema))
	sentimentStage.Prompt = fmt.Sprintf(
		"You are a customer support sentiment analyzer. Determine the customer's sentiment and list any phrases that signal urgency.\n\nTicket:\n%s", ticket)

	prioritizeStage := orla.NewStage("prioritize", lightBackend)
	prioritizeStage.SetMaxTokens(256)
	prioritizeStage.SetTemperature(0)
	prioritizeStage.SetSchedulingPolicy(orla.SchedulingPolicyFCFS)
	prioritizeStage.SetResponseFormat(orla.NewStructuredOutputRequest("ticket_priority", prioritySchema))
	prioritizeStage.SetPromptBuilder(func(results map[string]*orla.StageResult) (string, error) {
		classification, ok := results[classifyStage.ID]
		if !ok {
			return "", fmt.Errorf("missing classify stage result")
		}
		sentiment, ok := results[sentimentStage.ID]
		if !ok {
			return "", fmt.Errorf("missing sentiment stage result")
		}
		return fmt.Sprintf(
			"Given this ticket classification and sentiment analysis, assign a severity (critical / high / medium / low) and a scheduling priority from 1 (lowest) to 10 (highest). Explain why in one sentence.\n\nClassification:\n%s\n\nSentiment:\n%s",
			classification.Response.Content, sentiment.Response.Content), nil
	})

	if err := triage.AddStage(classifyStage); err != nil {
		return err
	}
	if err := triage.AddStage(sentimentStage); err != nil {
		return err
	}
	if err := triage.AddStage(prioritizeStage); err != nil {
		return err
	}
	if err := triage.AddDependency(prioritizeStage.ID, classifyStage.ID); err != nil {
		return err
	}
	if err := triage.AddDependency(prioritizeStage.ID, sentimentStage.ID); err != nil {
		return err
	}

	// --- Agent 2: resolver (heavy model, three-stage diamond DAG) ---
	//
	//   draft_response ──┐
	//                    ├──▶ final_review
	//   policy_check ────┘
	//
	resolver := orla.NewAgent(client)
	resolver.Name = "resolver"

	draftStage := orla.NewStage("draft_response", heavyBackend)
	draftStage.SetMaxTokens(1024)
	draftStage.SetTemperature(0.3)
	draftStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)

	policyStage := orla.NewStage("policy_check", heavyBackend)
	policyStage.SetMaxTokens(512)
	policyStage.SetTemperature(0)
	policyStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)

	reviewStage := orla.NewStage("final_review", heavyBackend)
	reviewStage.SetMaxTokens(512)
	reviewStage.SetTemperature(0)
	reviewStage.SetSchedulingPolicy(orla.SchedulingPolicyPriority)
	reviewStage.SetPromptBuilder(func(results map[string]*orla.StageResult) (string, error) {
		draft, ok := results[draftStage.ID]
		if !ok {
			return "", fmt.Errorf("missing draft_response stage result")
		}
		policies, ok := results[policyStage.ID]
		if !ok {
			return "", fmt.Errorf("missing policy_check stage result")
		}
		return fmt.Sprintf(
			"You are a QA reviewer for customer support responses. Review this draft reply against the applicable policies below. Check for: (1) policy compliance, (2) professional tone, (3) completeness. If it passes, output APPROVED followed by a brief summary. If not, explain what to fix.\n\nDraft Reply:\n%s\n\nApplicable Policies:\n%s",
			draft.Response.Content, policies.Response.Content), nil
	})

	if err := resolver.AddStage(draftStage); err != nil {
		return err
	}
	if err := resolver.AddStage(policyStage); err != nil {
		return err
	}
	if err := resolver.AddStage(reviewStage); err != nil {
		return err
	}
	if err := resolver.AddDependency(reviewStage.ID, draftStage.ID); err != nil {
		return err
	}
	if err := resolver.AddDependency(reviewStage.ID, policyStage.ID); err != nil {
		return err
	}

	// --- Agent 3: escalation (light model, single stage; parallel with resolver) ---
	escalation := orla.NewAgent(client)
	escalation.Name = "escalation"

	routeStage := orla.NewStage("route_ticket", lightBackend)
	routeStage.SetMaxTokens(256)
	routeStage.SetTemperature(0)
	routeStage.SetSchedulingPolicy(orla.SchedulingPolicyFCFS)
	routeStage.SetResponseFormat(orla.NewStructuredOutputRequest("ticket_escalation", escalationSchema))

	if err := escalation.AddStage(routeStage); err != nil {
		return err
	}

	// --- Stage Mapping (validation) ---
	allStages := []*orla.Stage{classifyStage, sentimentStage, prioritizeStage, draftStage, policyStage, reviewStage, routeStage}
	mapping := &orla.ExplicitStageMapping{}
	output, err := mapping.Map(&orla.StageMappingInput{
		Stages:   allStages,
		Backends: []*orla.LLMBackend{lightBackend, heavyBackend},
	})
	if err != nil {
		return fmt.Errorf("stage mapping: %w", err)
	}
	if err := orla.ApplyStageMappingOutput(allStages, output); err != nil {
		return fmt.Errorf("apply stage mapping: %w", err)
	}
	log.Printf("Stage mapping validated: %d stages assigned to backends", len(output.Assignments))

	// --- Workflow ---
	wf := orla.NewWorkflow()
	if err := wf.AddAgent(triage); err != nil {
		return err
	}
	if err := wf.AddAgent(resolver); err != nil {
		return err
	}
	if err := wf.AddAgent(escalation); err != nil {
		return err
	}
	if err := wf.AddDependency("resolver", "triage"); err != nil {
		return err
	}
	if err := wf.AddDependency("escalation", "triage"); err != nil {
		return err
	}

	// Context passing: feed triage output into the resolver and escalation agents.
	// Both run in parallel after triage completes.
	wf.SetContextPassingFn(func(upstreamResults map[string]*orla.AgentResult, downstream *orla.Agent) error {
		triageResult, ok := upstreamResults["triage"]
		if !ok {
			return nil
		}

		var classifyOutput, sentimentOutput, prioritizeOutput string
		if cr, ok := triageResult.StageResults[classifyStage.ID]; ok && cr.Response != nil {
			classifyOutput = cr.Response.Content
		}
		if sr, ok := triageResult.StageResults[sentimentStage.ID]; ok && sr.Response != nil {
			sentimentOutput = sr.Response.Content
		}
		if pr, ok := triageResult.StageResults[prioritizeStage.ID]; ok && pr.Response != nil {
			prioritizeOutput = pr.Response.Content
		}

		var triageOutput string
		for _, sr := range triageResult.StageResults {
			if sr.Response != nil {
				triageOutput += sr.Response.Content + "\n"
			}
		}

		priority := 5
		if prioritizeOutput != "" {
			var p struct {
				Priority int `json:"priority"`
			}
			if err := json.Unmarshal([]byte(prioritizeOutput), &p); err == nil && p.Priority > 0 {
				priority = p.Priority
			}
		}

		switch downstream.Name {
		case "resolver":
			log.Printf("Triage assigned scheduling priority %d to resolver", priority)
			stages := downstream.Stages()
			for _, s := range stages {
				s.SetSchedulingHints(&orla.SchedulingHints{Priority: &priority})
				switch s.Name {
				case "draft_response":
					s.Prompt = fmt.Sprintf(
						"You are a customer support agent. Write a helpful, professional reply to this customer based on the triage analysis below. Address their specific issue, apologize if appropriate, and provide clear next steps.\n\nTriage Analysis:\n%s\n\nOriginal Ticket:\n%s",
						triageOutput, ticket)
				case "policy_check":
					s.Prompt = fmt.Sprintf(
						"You are a support policy specialist. Given the ticket classification and sentiment below, identify all applicable support policies, refund rules, SLA commitments, and any constraints the response agent must follow.\n\nClassification:\n%s\n\nSentiment:\n%s\n\nOriginal Ticket:\n%s",
						classifyOutput, sentimentOutput, ticket)
				}
			}

		case "escalation":
			stages := downstream.Stages()
			for _, s := range stages {
				if s.Name == "route_ticket" {
					s.Prompt = fmt.Sprintf(
						"You are a support escalation router. Based on the triage analysis below, decide whether this ticket requires human escalation. Consider the severity, customer sentiment, and whether the issue can be resolved automatically.\n\nClassification:\n%s\n\nSentiment:\n%s\n\nPriority Assessment:\n%s\n\nOriginal Ticket:\n%s",
						classifyOutput, sentimentOutput, prioritizeOutput, ticket)
				}
			}
		}
		return nil
	})

	// --- Execute ---
	log.Println("Executing customer support workflow...")
	results, err := wf.Execute(ctx)
	if err != nil {
		return fmt.Errorf("workflow execution: %w", err)
	}

	// --- Print results ---
	for agentName, agentResult := range results {
		log.Printf("=== Agent: %s ===", agentName)
		for stageID, stageResult := range agentResult.StageResults {
			log.Printf("  Stage %s:", stageID)
			if stageResult.Response != nil {
				content := stageResult.Response.Content
				if len(content) > 500 {
					content = content[:500] + "..."
				}
				log.Printf("    %s", content)
			}
		}
	}

	return nil
}

// SampleTicket is an example customer support ticket for running the demo.
const SampleTicket = `Subject: Charged twice for my subscription - URGENT

Hi,

I just noticed that my credit card was charged $49.99 TWICE for my Pro
subscription this month (Oct 3 and Oct 5). I only have one account and
I definitely did not sign up for a second subscription.

I've been a customer for 2 years and this has never happened before.
I need a refund for the duplicate charge ASAP - I'm on a tight budget
this month and that extra $50 really hurts.

Also, while I have your attention - the dashboard has been loading
really slowly for the past week. Sometimes it takes 30+ seconds.
Is there something going on with the servers?

Thanks,
Alex Johnson
Account: alex.johnson@email.com
Plan: Pro ($49.99/month)`
