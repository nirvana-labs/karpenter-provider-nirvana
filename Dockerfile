FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /karpenter-provider-nirvana ./cmd/controller

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /karpenter-provider-nirvana /karpenter-provider-nirvana
USER non-root
ENTRYPOINT ["/karpenter-provider-nirvana"]
