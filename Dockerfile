# Multi-stage Dockerfile
FROM golang:1.22 AS builder

WORKDIR /workspace

COPY go.mod go.sum* ./
RUN if [ -f go.mod ]; then go mod download; fi

COPY . .

ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o /workspace/govel-syncio ./src

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /workspace/govel-syncio /app/govel-syncio
COPY --from=builder /workspace/assets /app/assets
COPY --from=builder /workspace/views /app/views
COPY --from=builder /workspace/manifest.json /app/manifest.json

RUN mkdir -p /app/data

EXPOSE 50051 8080

ENTRYPOINT ["/app/govel-syncio"]
