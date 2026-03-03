# SWE-bench Lite experiments image. Build from repo root:
#   docker build -f examples/swe_bench_lite/Dockerfile .
# Run with EXPERIMENT_MODE env var to select mode (baseline, stage_mapping, sjf).
# Dataset is committed as dataset.zip; unzipped at build time to /dataset.

FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /build/single_shot ./examples/swe_bench_lite/cmd/single_shot

FROM alpine:3.21
RUN apk add --no-cache ca-certificates bash curl git jq make patch py3-pip python3 unzip
COPY --from=builder /build/single_shot /usr/local/bin/single_shot
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
