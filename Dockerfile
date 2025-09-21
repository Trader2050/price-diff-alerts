# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/usdewatcher ./cmd/usdewatcher

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/usdewatcher /usr/local/bin/usdewatcher
COPY config.example.yaml /app/config.yaml

ENTRYPOINT ["/usr/local/bin/usdewatcher"]
CMD ["run", "--config", "/app/config.yaml"]
