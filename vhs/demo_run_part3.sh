#!/bin/bash
# Demo runner for Part 3: Priority + flush at boundary (two workflows).
# Usage: ./vhs/demo_run_part3.sh (from repo root)

set -e

bold="\033[1m"
reset="\033[0m"

echo ""
echo -e "${bold}Orla Part 3: Priority + Flush at Boundary (Two Workflows)${reset}"
echo ""
echo "  Light backend: Qwen3-4B (classify)"
echo "  Heavy backend: Qwen3-8B (policy_check, reply, route_ticket)"
echo ""
sleep 3

echo -e "${bold}Ticket: Billing Dispute${reset}"
echo ""
batcat --style=plain --paging=never examples/workflow_demo/tickets/billing.txt 2>/dev/null || cat examples/workflow_demo/tickets/billing.txt
sleep 8
echo ""
TICKET_PATH=examples/workflow_demo/tickets/billing.txt go run ./examples/demo_video/cmd/part3
sleep 6

echo ""
echo -e "${bold}Part 3 complete.${reset}"
