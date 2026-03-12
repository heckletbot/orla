#!/bin/bash
# Demo runner for Part 2: Priority scheduling + cache preservation.
# Usage: ./vhs/demo_run_part2.sh (from repo root)

set -e

bold="\033[1m"
reset="\033[0m"

echo ""
echo -e "${bold}Orla Part 2: Priority Scheduling + Cache Preservation${reset}"
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
TICKET_PATH=examples/workflow_demo/tickets/billing.txt go run ./examples/demo_video/cmd/part2
sleep 6

echo ""
echo -e "${bold}Part 2 complete.${reset}"
