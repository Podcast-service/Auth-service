FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY docker .

RUN go build -o auth_service ./cmd/auth_service/main.go

FROM alpine:latest AS runtime
WORKDIR /app
COPY --from=builder /app/auth_service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./auth_service"]
