# SWE-bench Lite experiments image. Build from repo root:
#   docker build -f examples/swe_bench_lite/Dockerfile .
# Run with make target as command, e.g.: docker run ... baseline
# Dataset is committed as dataset.zip; unzipped at build time to /dataset.

FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build each experiment binary; add more lines as experiments are added.
RUN CGO_ENABLED=0 go build -o /build/baseline ./examples/swe_bench_lite/cmd/baseline
RUN CGO_ENABLED=0 go build -o /build/two_stage_mapping ./examples/swe_bench_lite/cmd/two_stage_mapping

FROM alpine:3.21
RUN apk add --no-cache ca-certificates bash curl git jq make patch py3-pip python3 unzip
COPY --from=builder /build/baseline /usr/local/bin/baseline
COPY --from=builder /build/two_stage_mapping /usr/local/bin/two_stage_mapping
COPY examples/swe_bench_lite/dataset.zip /tmp/dataset.zip
RUN unzip -o /tmp/dataset.zip -d /dataset && rm /tmp/dataset.zip
COPY examples/swe_bench_lite/Makefile /Makefile
COPY examples/swe_bench_lite/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV PATH="/usr/local/bin:$PATH"
ENV ORLA_URL=http://orla:8081
ENV WORKDIR_ROOT=/workdir
ENV OUTPUT_PATH=/output/predictions.jsonl
WORKDIR /

ENTRYPOINT ["/entrypoint.sh"]
