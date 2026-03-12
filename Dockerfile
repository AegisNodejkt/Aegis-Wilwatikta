# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the CLI application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /aegis-reviewer ./cmd/cli

# Stage 2: Create the final lightweight image
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /aegis-reviewer .

# Set the entrypoint for the container
ENTRYPOINT ["/root/aegis-reviewer"]
