# CAIS 2026 Demo: Orla Customer Support Workflow
#
# Pre-recording requirements:
#   1. SGLang backends must be running:
#      docker compose -f deploy/docker-compose.workflow-demo.yaml up -d
#   2. Wait for health checks to pass (both SGLang models + Orla server)
#   3. chmod +x vhs/demo_run.sh
#   4. Run from the repo root: vhs vhs/demo.tape
#
# Timing: The main Sleep must be longer than the total runtime. Do a dry run
#         of vhs/demo_run.sh first and adjust accordingly.

Output share/demo.mp4

Require tmux
Require curl

Set Theme "3024 Night"
Set FontSize 20
Set Width 1800
Set Height 1100
Set LineHeight 1.3

# =========================================================================
# Segment 1: Setup — start backends and verify health
# =========================================================================

Type "docker compose -f deploy/docker-compose.workflow-demo.yaml up -d"
Enter
Sleep 8s

Type "curl -s http://localhost:8081/api/v1/health | jq"
Enter
Sleep 3s

# =========================================================================
# Segment 2: Start tmux with Orla server logs in bottom pane (vertical split)
# =========================================================================

Type "tmux new-session -d -s demo"
Enter
Sleep 1s

# Vertical split: top = demo, bottom = logs (each pane gets full width)
Type "tmux split-window -v -t demo"
Enter
Sleep 1s

# Bottom pane (0.1): Orla server logs (--tail=0 = only new lines)
Type 'tmux send-keys -t demo:0.1 "cd ~/orla && docker compose -f deploy/docker-compose.workflow-demo.yaml logs -f --tail=0 orla" Enter'
Enter
Sleep 4s

# Select top pane (0.0) so attach puts us there for the demo
Type "tmux select-pane -t demo:0.0"
Enter
Sleep 1s

# Attach to the tmux session (left pane is now active)
Type "tmux attach -t demo"
Enter
Sleep 2s

# =========================================================================
# Segment 3: Run both tickets via the demo script
# =========================================================================

Type "./vhs/demo_run.sh"
Enter
Sleep 50s

