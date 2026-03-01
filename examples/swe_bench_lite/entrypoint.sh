#!/bin/sh
set -e
# Runs the make target from RUN_TARGET (when using docker compose up run) or from arguments (docker compose run run <target>).
#   RUN_TARGET=two_stage_mapping docker compose -f deploy/docker-compose.swebench-lite.yaml up run
#   docker compose run --rm run baseline
mkdir -p "$(dirname "$OUTPUT_PATH")"
TARGET="${RUN_TARGET:-baseline}"
echo "RUN_TARGET=${RUN_TARGET:-<unset>} => exec make $TARGET"
if [ $# -ge 1 ]; then
  exec make "$@"
else
  exec make "$TARGET"
fi
