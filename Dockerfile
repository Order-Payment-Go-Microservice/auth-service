FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o auth-service ./cmd

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/auth-service .
EXPOSE 50052
CMD ["./auth-service"]
