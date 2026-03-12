#!/bin/bash
# Demo runner for Part 1: Stage mapping only.
# Usage: ./vhs/demo_run_part1.sh (from repo root)

set -e

bold="\033[1m"
reset="\033[0m"

echo ""
echo -e "${bold}Orla Part 1: Stage Mapping Only${reset}"
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
TICKET_PATH=examples/workflow_demo/tickets/billing.txt go run ./examples/demo_video/cmd/part1
sleep 6

echo ""
echo -e "${bold}Part 1 complete.${reset}"
