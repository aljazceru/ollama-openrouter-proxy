FROM golang:latest AS builder
WORKDIR /app

# Install build dependencies for CGO/SQLite
RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO for SQLite support
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o ollama-proxy


FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/ollama-proxy /ollama-proxy

EXPOSE 11434
ENTRYPOINT ["/ollama-proxy"]