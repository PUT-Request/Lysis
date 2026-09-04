package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const UserContextKey contextKey = "user"

type UserClaims struct {
	UserID int
	Email  string
}

func Middleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := ValidateToken(parts[1], jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			uc := &UserClaims{
				UserID: claims.UserID,
				Email:  claims.Email,
			}
			ctx := context.WithValue(r.Context(), UserContextKey, uc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(r *http.Request) *UserClaims {
	uc, ok := r.Context().Value(UserContextKey).(*UserClaims)
	if !ok {
		return nil
	}
	return uc
}
