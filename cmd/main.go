package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/Order-Payment-Go-Microservice/auth-service/config"
	grpcHandler "github.com/Order-Payment-Go-Microservice/auth-service/internal/delivery/grpc"
	"github.com/Order-Payment-Go-Microservice/auth-service/internal/usecase"
	jwtpkg "github.com/Order-Payment-Go-Microservice/auth-service/pkg/jwt"
	authv1 "github.com/Order-Payment-Go-Microservice/proto-generation/gen/auth/v1"
	userv1 "github.com/Order-Payment-Go-Microservice/proto-generation/gen/user/v1"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	log.Println("connected to redis")

	// User-service gRPC client
	userAddr := fmt.Sprintf("%s:%s", cfg.UserService.Host, cfg.UserService.Port)
	userConn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial user-service: %v", err)
	}
	defer userConn.Close()
	userClient := userv1.NewUserServiceClient(userConn)

	// JWT manager
	jwtManager := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL)

	// Usecase + handler
	authUC := usecase.NewAuthUsecase(userClient, jwtManager, rdb)
	handler := grpcHandler.NewAuthHandler(authUC)

	// gRPC server
	grpcServer := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	log.Printf("auth-service gRPC listening on :%s", cfg.GRPC.Port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
