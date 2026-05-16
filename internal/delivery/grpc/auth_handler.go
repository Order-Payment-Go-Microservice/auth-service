package grpc

import (
	"context"
	"errors"

	"github.com/Order-Payment-Go-Microservice/auth-service/internal/usecase"
	"github.com/Order-Payment-Go-Microservice/auth-service/pkg/jwt"
	authv1 "github.com/Order-Payment-Go-Microservice/proto-generation/gen/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	uc usecase.AuthUsecase
}

func NewAuthHandler(uc usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	pair, user, err := h.uc.Register(ctx, usecase.RegisterRequest{
		Username:  req.Username,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.RegisterResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toProtoUser(user),
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	pair, user, err := h.uc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.LoginResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         toProtoUser(user),
	}, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	pair, err := h.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.RefreshTokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := h.uc.Logout(ctx, req.RefreshToken); err != nil {
		return nil, mapError(err)
	}
	return &authv1.LogoutResponse{Success: true}, nil
}

func (h *AuthHandler) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	claims, err := h.uc.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.ValidateTokenResponse{
		UserId: claims.UserID,
		Email:  claims.Email,
	}, nil
}

func (h *AuthHandler) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	if err := h.uc.ChangePassword(ctx, req.UserId, req.OldPassword, req.NewPassword); err != nil {
		return nil, mapError(err)
	}
	return &authv1.ChangePasswordResponse{Success: true}, nil
}

func toProtoUser(u *usecase.UserInfo) *authv1.AuthUser {
	return &authv1.AuthUser{
		Id:         u.ID,
		Username:   u.Username,
		Email:      u.Email,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Avatar:     u.Avatar,
		IsVerified: u.IsVerified,
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, usecase.ErrTokenBlacklisted):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, usecase.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, jwt.ErrInvalidToken), errors.Is(err, jwt.ErrExpiredToken):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
