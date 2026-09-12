package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GetAccessSecret() []byte {
	got := []byte(os.Getenv("JWT_SECRET_ACCESS"))
	if len(got) == 0 {
		return []byte("access_token")
	}
	return got
}

func GetRefreshSecret() []byte {
	got := []byte(os.Getenv("JWT_SECRET_REFRESH"))
	if len(got) == 0 {
		return []byte("refresh_token")
	}
	return got
}

func GenerateAccessJWT(userID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(GetAccessSecret())
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func GenerateRefreshJWT(userID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(GetRefreshSecret())
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func parseJWT(signedToken string, secret []byte) (int, error) {
	token, err := jwt.Parse(signedToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return 0, err
	}
	if !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid claims")
	}

	userIDValue, ok := claims["user_id"]
	if !ok {
		return 0, fmt.Errorf("user_id not found")
	}

	userIDFloat, ok := userIDValue.(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user_id type")
	}

	return int(userIDFloat), nil
}

func ParseJWTAccess(signedToken string) (int, error) {
	return parseJWT(signedToken, GetAccessSecret())
}

func ParseJWTRefresh(signedToken string) (int, error) {
	return parseJWT(signedToken, GetRefreshSecret())
}
