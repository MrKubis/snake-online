package matchmaker

import "os"

type Config struct {
	RedisAddr string
}

func ConfigFromEnv() Config {
	return Config{
		RedisAddr: getenv("REDDIS_ADDR", "localhost:6379"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
