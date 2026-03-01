#!/bin/sh
set -e
# Runs the make target from RUN_TARGET (default: baseline). Set RUN_TARGET when starting the stack, e.g. RUN_TARGET=two_stage_mapping docker compose up.
mkdir -p "$(dirname "$OUTPUT_PATH")"
TARGET="${RUN_TARGET:-baseline}"
echo "exec make $TARGET"
exec make "$TARGET"
