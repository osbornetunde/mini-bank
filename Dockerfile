# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main ./cmd/bank/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Create a non-root user
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder --chown=appuser:appuser /app/main .

# Switch to non-root user
USER appuser

# Create entrypoint or just run main

# Note: Environment variables should be passed via docker-compose.yml or -e flags
# DO NOT copy .env files into the Docker image

EXPOSE 8080

CMD ["./main"]
