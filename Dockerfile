FROM golang:1.25-alpine AS builder

RUN apk add --no-cache protobuf protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /app

# Generate proto stubs from the shared proto-generation module
COPY proto-generation/go.mod /app/proto-generation/go.mod
COPY proto-generation/gen /app/proto-generation/gen

RUN cd /app/proto-generation && \
    protoc -I . -I /usr/include \
        --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        gen/auth/v1/auth.proto && \
    protoc -I . -I /usr/include \
        --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        gen/user/v1/user.proto

WORKDIR /app/auth-service
COPY auth-service/go.mod auth-service/go.sum ./

# Switch to local proto-generation before downloading deps
RUN go mod edit -replace github.com/Order-Payment-Go-Microservice/proto-generation=../proto-generation
RUN go mod download

COPY auth-service/ .
RUN go build -o auth-service ./cmd

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/auth-service/auth-service .
EXPOSE 50055
CMD ["./auth-service"]
