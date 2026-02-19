package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/arko-chat/arko/internal/credentials"
)

const (
	appName    = "arko"
	configFile = "config.json"
)

type Config struct {
	CryptoDBPath  string `json:"crypto_db_path"`
	SessionSecret string `json:"-"`
	PickleKey     string `json:"-"`
}

func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, appName)

	path := filepath.Join(appDir, configFile)
	var cfg Config

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	} else {
		cfg.CryptoDBPath = filepath.Join(appDir, "crypto")
		if err := os.MkdirAll(appDir, 0700); err != nil {
			return nil, err
		}
		out, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(path, out, 0600)
		log.Printf("Generated new config at: %s", path)
	}

	cfg.SessionSecret, err = credentials.LoadAppSecret("session_secret")
	if err != nil {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		cfg.SessionSecret = base64.StdEncoding.EncodeToString(secret)
		if err := credentials.StoreAppSecret("session_secret", cfg.SessionSecret); err != nil {
			return nil, err
		}
	}

	cfg.PickleKey, err = credentials.LoadAppSecret("pickle_key")
	if err != nil {
		pickle := make([]byte, 32)
		if _, err := rand.Read(pickle); err != nil {
			return nil, err
		}
		cfg.PickleKey = base64.StdEncoding.EncodeToString(pickle)
		if err := credentials.StoreAppSecret("pickle_key", cfg.PickleKey); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CRYPTO_DB_PATH"); v != "" {
		cfg.CryptoDBPath = v
	}
	if v := os.Getenv("SESSION_SECRET"); v != "" {
		cfg.SessionSecret = v
	}
	if v := os.Getenv("PICKLE_KEY"); v != "" {
		cfg.PickleKey = v
	}
}
