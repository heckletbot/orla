# 5-Minute Demo Variants

Three separate entry points for the Orla customer support ticket demo. Each variant highlights different Orla features—stage mapping, scheduling policies, and cache management—so you can show them one by one in a demo video.

All three parts use the same underlying workflow: a customer support ticket triage and resolution pipeline with stages for classification, policy lookup, reply drafting, and ticket routing. The differences are in how Orla is configured: which scheduling policy is used, whether the KV cache is preserved or flushed, and whether a second workflow runs to demonstrate flush-at-boundary behavior.

Start the workflow demo stack from the repo root with `docker compose -f deploy/docker-compose.workflow-demo.yaml up -d`. This brings up Orla, SGLang (heavy model), and SGLang-light. Wait for the services to be healthy before running the demo clients.

**Part 1** runs with `go run ./examples/demo_video/cmd/part1`. It builds the customer support workflow exactly as in `workflow_demo`, with only stage mapping (classify on light, policy/reply/route on heavy). No scheduling or memory policy customization. The key config lives in `part1/part1.go`.

**Part 2** runs with `go run ./examples/demo_video/cmd/part2`. Same stage mapping, but priority scheduling instead of FCFS, and the default memory policy so the cache is preserved when stages stay on the same backend with small token deltas. See `part2/part2.go`.

**Part 3** runs with `go run ./examples/demo_video/cmd/part3`. Same as Part 2, plus a second small workflow after the main one. Orla flushes the cache at the workflow boundary between them, so the second workflow starts with a clean cache. See `part3/part3.go` for `RunSecondWorkflow: true`.

VHS tapes render the demos: `vhs vhs/demo_part1.tape`, `vhs vhs/demo_part2.tape`, `vhs vhs/demo_part3.tape`. Output goes to `share/demo_part1.mp4`, etc.