FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY proto-generation/ /app/proto-generation/

WORKDIR /app/auth-service
COPY auth-service/go.mod auth-service/go.sum ./
RUN go mod edit -replace github.com/Order-Payment-Go-Microservice/proto-generation=../proto-generation
RUN go mod download

COPY auth-service/ .
RUN go build -o auth-service ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/auth-service/auth-service .
EXPOSE 50053
CMD ["./auth-service"]
