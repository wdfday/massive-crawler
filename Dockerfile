# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main ./cmd/us-data/

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/main .

# Default indices (fallback if volume not mounted); app prefers volume-mounted indices
COPY --from=builder /app/indices ./indices

RUN mkdir -p /app/data

ENV DATA_DIR=/app/data

CMD ["./main"]
