package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func (s *Server) issueToken(playerId string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   playerId,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Server) parseToken(raw string) (string, error) {
	var claims jwt.RegisteredClaims

	tok, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return s.jwtSecret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		slog.Error("Error parsing token", "err", err)
		return "", fmt.Errorf("error parsing token")
	}

	if !tok.Valid {
		slog.Error("Invalid token", "token", tok)
		return "", fmt.Errorf("invalid token")
	}

	if claims.Subject == "" {
		slog.Error("Empty claims for token", "token", tok)
		return "", fmt.Errorf("Empty claims for token")
	}
	return claims.Subject, nil
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id, err := s.parseToken(raw)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), PlayerIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
