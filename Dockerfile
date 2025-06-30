# 1. Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/server ./cmd/server

# 2. Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /app/server /server
COPY --from=builder /app/.env /.env

USER nonroot:nonroot

ENTRYPOINT ["/server"]
