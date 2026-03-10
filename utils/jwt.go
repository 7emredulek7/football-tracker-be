package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var GetJWTSecret = func() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "supersecretjwtkey"
	}
	return []byte(secret)
}

func GenerateToken(userID string, role string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userID,
		"role":   role,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	return token.SignedString(GetJWTSecret())
}
