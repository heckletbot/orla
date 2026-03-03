# SWE-bench Lite experiments. In Docker the binary is at /usr/local/bin (on PATH); entrypoint runs "make <target>".
# Locally: EXPERIMENT_MODE=baseline go run ./examples/swe_bench_lite/cmd/single_shot

single_shot_baseline:
	EXPERIMENT_MODE=baseline exec single_shot

single_shot_stage_mapping:
	EXPERIMENT_MODE=stage_mapping exec single_shot

single_shot_sjf:
	EXPERIMENT_MODE=sjf exec single_shot
