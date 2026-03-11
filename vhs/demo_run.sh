#!/bin/bash
# Demo runner for CAIS 2026 video recording.
# Runs both customer support tickets through Orla sequentially.
# Usage: ./vhs/demo_run.sh (from repo root)

set -e

echo ""
echo "Orla Customer Support Workflow"
echo ""
echo "  Light backend: Qwen3-4B (classify)"
echo "  Heavy backend: Qwen3-8B (policy_check, reply, route_ticket)"
echo ""

# --- Ticket 1 ---
echo ""
echo "Ticket 1: Billing Dispute"
echo ""
cat examples/workflow_demo/tickets/billing.txt
echo ""
TICKET_PATH=examples/workflow_demo/tickets/billing.txt go run ./examples/workflow_demo/cmd/workflow_demo

echo ""

# --- Ticket 2 ---
echo ""
echo "Ticket 2: Account Compromise"
echo ""
cat examples/workflow_demo/tickets/account_compromise.txt
echo ""
TICKET_PATH=examples/workflow_demo/tickets/account_compromise.txt go run ./examples/workflow_demo/cmd/workflow_demo

echo ""
echo "Demo complete."
