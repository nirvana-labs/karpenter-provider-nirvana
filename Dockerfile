# syntax=docker/dockerfile:1.7
#
# Multi-stage image used for dev, PR, and snapshot builds (compiles from source).
# Tagged releases use ./Dockerfile.release, which copies the goreleaser-built
# binary into distroless. Keep the distroless base digest, labels, and runtime
# env in sync between the two files.

# golang:1.26.1-alpine3.22
FROM golang@sha256:595c7847cff97c9a9e76f015083c481d26078f961c9c8dca3923132f51fe12f1 AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GIT_SHA=dev

WORKDIR /src

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Keep this list in sync with top-level Go package directories used by the controller.
COPY cmd ./cmd

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${GIT_SHA}" \
        -o /out/karpenter-provider-nirvana \
        ./cmd/controller

# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1

ARG GIT_SHA=dev

LABEL org.opencontainers.image.title="karpenter-provider-nirvana" \
      org.opencontainers.image.description="Karpenter provider implementation for Nirvana Labs" \
      org.opencontainers.image.source="https://github.com/nirvana-labs/karpenter-provider-nirvana" \
      org.opencontainers.image.revision="${GIT_SHA}"

ENV NIRVANA_RELEASE=${GIT_SHA}

COPY --from=builder /out/karpenter-provider-nirvana /karpenter-provider-nirvana

USER nonroot:nonroot

ENTRYPOINT ["/karpenter-provider-nirvana"]
