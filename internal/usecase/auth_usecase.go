package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Order-Payment-Go-Microservice/auth-service/pkg/jwt"

	userv1 "github.com/Order-Payment-Go-Microservice/proto-generation/gen/user/v1"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenBlacklisted   = errors.New("token has been revoked")
	ErrUserNotFound       = errors.New("user not found")
)

const blacklistPrefix = "auth:blacklist:"

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type UserInfo struct {
	ID         string
	Username   string
	Email      string
	FirstName  string
	LastName   string
	Avatar     string
	IsVerified bool
}

type RegisterRequest struct {
	Username  string
	Email     string
	Password  string
	FirstName string
	LastName  string
}

type AuthUsecase interface {
	Register(ctx context.Context, req RegisterRequest) (*TokenPair, *UserInfo, error)
	Login(ctx context.Context, email, password string) (*TokenPair, *UserInfo, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
	ValidateToken(ctx context.Context, accessToken string) (*jwt.Claims, error)
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
}

type authUsecase struct {
	userClient userv1.UserServiceClient
	jwtManager *jwt.Manager
	redis      *redis.Client
}

func NewAuthUsecase(userClient userv1.UserServiceClient, jwtManager *jwt.Manager, rdb *redis.Client) AuthUsecase {
	return &authUsecase{
		userClient: userClient,
		jwtManager: jwtManager,
		redis:      rdb,
	}
}

func (u *authUsecase) Register(ctx context.Context, req RegisterRequest) (*TokenPair, *UserInfo, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	resp, err := u.userClient.CreateUser(ctx, &userv1.CreateUserRequest{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
	})
	if err != nil {
		return nil, nil, err
	}

	pair, err := u.generatePair(resp.User.Id, resp.User.Email)
	if err != nil {
		return nil, nil, err
	}

	info := protoToUserInfo(resp.User)
	return pair, info, nil
}

func (u *authUsecase) Login(ctx context.Context, email, password string) (*TokenPair, *UserInfo, error) {
	resp, err := u.userClient.GetUserByEmail(ctx, &userv1.GetUserByEmailRequest{Email: email})
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(resp.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := u.generatePair(resp.User.Id, resp.User.Email)
	if err != nil {
		return nil, nil, err
	}

	info := protoToUserInfo(resp.User)
	return pair, info, nil
}

func (u *authUsecase) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := u.jwtManager.Validate(refreshToken)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "refresh" {
		return nil, jwt.ErrInvalidToken
	}

	if u.isBlacklisted(ctx, refreshToken) {
		return nil, ErrTokenBlacklisted
	}

	pair, err := u.generatePair(claims.UserID, claims.Email)
	if err != nil {
		return nil, err
	}

	// blacklist old refresh token
	_ = u.blacklist(ctx, refreshToken, claims.ExpiresAt.Time)

	return pair, nil
}

func (u *authUsecase) Logout(ctx context.Context, refreshToken string) error {
	claims, err := u.jwtManager.Validate(refreshToken)
	if err != nil {
		return err
	}
	if claims.TokenType != "refresh" {
		return jwt.ErrInvalidToken
	}
	return u.blacklist(ctx, refreshToken, claims.ExpiresAt.Time)
}

func (u *authUsecase) ValidateToken(ctx context.Context, accessToken string) (*jwt.Claims, error) {
	claims, err := u.jwtManager.Validate(accessToken)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "access" {
		return nil, jwt.ErrInvalidToken
	}
	return claims, nil
}

func (u *authUsecase) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	resp, err := u.userClient.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
	if err != nil {
		return ErrUserNotFound
	}

	emailResp, err := u.userClient.GetUserByEmail(ctx, &userv1.GetUserByEmailRequest{Email: resp.User.Email})
	if err != nil {
		return ErrUserNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(emailResp.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = u.userClient.UpdatePassword(ctx, &userv1.UpdatePasswordRequest{
		UserId:       userID,
		PasswordHash: string(hash),
	})
	return err
}

func (u *authUsecase) generatePair(userID, email string) (*TokenPair, error) {
	access, err := u.jwtManager.GenerateAccessToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	refresh, err := u.jwtManager.GenerateRefreshToken(userID, email)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (u *authUsecase) blacklist(ctx context.Context, token string, expiry time.Time) error {
	ttl := time.Until(expiry)
	if ttl <= 0 {
		return nil
	}
	return u.redis.Set(ctx, blacklistPrefix+token, "1", ttl).Err()
}

func (u *authUsecase) isBlacklisted(ctx context.Context, token string) bool {
	val, err := u.redis.Get(ctx, blacklistPrefix+token).Result()
	return err == nil && val == "1"
}

func protoToUserInfo(u *userv1.User) *UserInfo {
	return &UserInfo{
		ID:         u.Id,
		Username:   u.Username,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Avatar:     u.Avatar,
		IsVerified: u.IsVerified,
	}
}
