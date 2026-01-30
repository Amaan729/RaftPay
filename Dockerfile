# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod (go.sum will be generated if missing)
COPY go.mod ./

# Download dependencies (this will create go.sum)
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o raftpay .

# Runtime stage
FROM alpine:latest

# Add ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/raftpay .

# Create data directory for persistence
RUN mkdir -p /app/data

# Expose Raft port and API port
EXPOSE 8000 9000

# Run the node
ENTRYPOINT ["./raftpay"]
