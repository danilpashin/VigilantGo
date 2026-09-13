FROM golang:1.27-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/vigilant-go ./cmd/api/

FROM alpine:3.23

RUN adduser -D appuser
USER appuser

WORKDIR /app

COPY --from=builder /app/vigilant-go ./vigilant-go