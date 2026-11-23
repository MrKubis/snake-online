package api

import (
	"os"
	"strconv"
)

type ServerMode string

const (
	ApiMode       ServerMode = "api"
	WebSocketMode ServerMode = "ws"
	AllMode       ServerMode = "all"
)

type Config struct {
	RedisAddr     string
	Addr          string
	JWTSecret     string
	ServerMode    ServerMode
	RunMatchmaker bool
}

func ConfigFromEnv() Config {
	runMatchmaker, err := strconv.ParseBool(os.Getenv("RUN_MATCHMAKER"))
	if err != nil {
		panic(err)
	}
	return Config{
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		Addr:          getenv("API_ADDR", ":3000"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		ServerMode:    ServerMode(getenv("SERVER_MODE", string(AllMode))),
		RunMatchmaker: runMatchmaker,
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
