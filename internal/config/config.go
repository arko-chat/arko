package config

import (
	"os"
)

type Config struct {
	SessionSecret string
	CryptoDBPath  string
	PickleKey     string
}

func Load() *Config {
	return &Config{
		SessionSecret: getEnv("SESSION_SECRET", "change-me-please-32chars-long!!!"),
		CryptoDBPath:  getEnv("CRYPTO_DB_PATH", "./data/crypto"),
		PickleKey:     getEnv("PICKLE_KEY", "arko-default-pickle-key-change!!"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
