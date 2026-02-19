package config

import (
	"os"
	"time"
)

type Config struct {
	Host            string
	Port            string
	StaticDir       string
	SessionSecret   string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	CryptoDBPath    string
	PickleKey       string
}

func Load() *Config {
	return &Config{
		Host:            getEnv("HOST", "0.0.0.0"),
		Port:            getEnv("PORT", "8090"),
		StaticDir:       getEnv("STATIC_DIR", "web/static"),
		SessionSecret:   getEnv("SESSION_SECRET", "change-me-please-32chars-long!!!"),
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CryptoDBPath:    getEnv("CRYPTO_DB_PATH", "./data/crypto"),
		PickleKey:       getEnv("PICKLE_KEY", "arko-default-pickle-key-change!!"),
	}
}

func (c *Config) Addr() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
