package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	rdb       *redis.Client
	jwtSecret []byte
	hub       *Hub
}

func (s *Server) ApiRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/api/health", HandleHealth)
	s.mountAPI(r)
	return r
}

func (s *Server) WSRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/api/health", HandleHealth)
	s.mountWS(r)
	return r
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/api/health", HandleHealth)
	s.mountAPI(r)
	s.mountWS(r)
	return r
}

func (s *Server) mountAPI(r chi.Router) chi.Router {
	r.Post("/api/anon", s.handleAnon)

	r.Group(func(r chi.Router) {
		r.Use(s.auth)
		r.Get("/api/queue/status", s.handleGetQueueStatus)
	})
	return r
}

func (s *Server) mountWS(r chi.Router) chi.Router {
	r.Group(func(r chi.Router) {
		r.Use(s.auth)
		r.Get("/api/ws", s.handleWS)
	})

	return r
}

func NewServer(cfg Config) (*Server, error) {
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("Failed to connect to redis. Ping (%s): %w", cfg.RedisAddr, err)
	}

	slog.Info("Connected to redis", "addr", cfg.RedisAddr)

	return &Server{
		rdb:       rdb,
		jwtSecret: []byte(cfg.JWTSecret),
	}, nil
}

func (s *Server) handleAnon(w http.ResponseWriter, r *http.Request) {
	id := uuid.New().String()

	token, err := s.issueToken(id)
	if err != nil {
		slog.Error(err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"player_id": id,
		"token":     token,
	})
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("healthy"))
}

func playerID(r *http.Request) string {
	id, _ := r.Context().Value(PlayerIDKey).(string)
	return id
}

func (s *Server) handleGetQueueStatus(w http.ResponseWriter, r *http.Request) {
	queueStatus, err := s.GetQueueStatus(r.Context())
	if err != nil {
		slog.Error("get queue status failed", "err", err)
		writeInternalServerError(w)
		return
	}

	writeJSON(w, http.StatusOK, queueStatus)
}

func writeJSON(w http.ResponseWriter, httpCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(v)
}

func writeInternalServerError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, InternalServerErrorMessage)
}
