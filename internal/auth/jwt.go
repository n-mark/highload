package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret   []byte
	issuer   string
	audience string
}

func NewJWTManager(secret, issuer, audience string) *JWTManager {
	return &JWTManager{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
	}
}

func (m *JWTManager) GenerateToken(userID string, ttl time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    m.issuer,
		Audience:  []string{m.audience},
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) VerifyToken(tokenString string) (string, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	token, err := parser.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token")
	}

	validator := jwt.NewValidator(jwt.WithAudience(m.audience), jwt.WithIssuer(m.issuer))
	if err := validator.Validate(claims); err != nil {
		return "", err
	}

	return claims.Subject, nil
}
