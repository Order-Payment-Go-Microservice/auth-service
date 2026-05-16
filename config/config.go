package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPC        GRPCConfig
	Redis       RedisConfig
	JWT         JWTConfig
	UserService UserServiceConfig
}

type GRPCConfig struct {
	Port string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type UserServiceConfig struct {
	Host string
	Port string
}

func Load() *Config {
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	accessTTL, _ := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	refreshTTL, _ := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h")) // 7 days

	return &Config{
		GRPC: GRPCConfig{
			Port: getEnv("GRPC_PORT", "50052"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "change-me-in-production"),
			AccessTokenTTL:  accessTTL,
			RefreshTokenTTL: refreshTTL,
		},
		UserService: UserServiceConfig{
			Host: getEnv("USER_SERVICE_HOST", "localhost"),
			Port: getEnv("USER_SERVICE_PORT", "50051"),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
