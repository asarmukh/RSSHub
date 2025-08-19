FROM golang:1.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o rsshub ./cmd/rsshub

# final runtime container
FROM debian:bookworm-slim

WORKDIR /app
COPY --from=builder /app/rsshub .

CMD ["./rsshub"]
