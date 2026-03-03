#!/bin/sh
set -e
# Runs the make target from RUN_TARGET (default: single_shot_baseline).
# Set RUN_TARGET when starting the stack, e.g. RUN_TARGET=single_shot_sjf docker compose up.
mkdir -p "$(dirname "$OUTPUT_PATH")"
TARGET="${RUN_TARGET:-single_shot_baseline}"
echo "exec make $TARGET"
exec make "$TARGET"
