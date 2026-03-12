FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o ai-reviewer cmd/cli/main.go

FROM alpine:latest

RUN apk add --no-cache git

COPY --from=builder /app/ai-reviewer /usr/local/bin/ai-reviewer

ENTRYPOINT ["ai-reviewer"]
