# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build API service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/api ./cmd/api

# Build notifier service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/notifier ./cmd/notifier

# Build rating-worker service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/rating-worker ./cmd/rating-worker

# API service stage
FROM alpine:3.19 AS api

RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

COPY --from=builder /bin/api .
COPY migrations migrations/

EXPOSE 8080

CMD ["./api"]

# Notifier service stage
FROM alpine:3.19 AS notifier

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /bin/notifier .

CMD ["./notifier"]

# Rating-worker service stage
FROM alpine:3.19 AS rating-worker

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /bin/rating-worker .

CMD ["./rating-worker"]
