package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
    TokenValidity = 7 * 24 * time.Hour
)

type JwtClaims struct {
	UserId  string `json:"user_id"`
	IsSuper bool   `json:"is_super"`
	jwt.RegisteredClaims
}

func NewJwtClaims() *JwtClaims {
    claim := &JwtClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenValidity)),
        },
    }
    return claim
}

