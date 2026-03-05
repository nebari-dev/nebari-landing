# Build stage
FROM golang:1.25 AS builder

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
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/webapi .

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/webapi"]
