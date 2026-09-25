package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/priyansx01/corn-hub-clone/internal/auth"
	"github.com/priyansx01/corn-hub-clone/internal/domain"
)

func main() {
	claims := auth.Claims{
		LMSUserID: "a0000000-0000-0000-0000-000000000001",
		Role:      domain.RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "a0000000-0000-0000-0000-000000000001",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("dev-secret-change-in-prod"))
	fmt.Println(tokenString)
}
