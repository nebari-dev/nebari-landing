# Build stage
FROM golang:1.25@sha256:9006890ecba0a168034d99516084099ae3114d9f2b7d6572c77f2dde57ebc980 AS builder

# TARGETARCH is injected automatically by docker buildx for each platform leg
# (e.g. "amd64", "arm64"). Declaring it here makes it available within this stage.
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build — use TARGETARCH so the binary matches the platform being built for.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -o webapi ./cmd

# Final stage
FROM gcr.io/distroless/static:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6

WORKDIR /

COPY --from=builder /workspace/webapi .

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/webapi"]
