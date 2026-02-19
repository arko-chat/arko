package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

const (
	appName    = "arko"
	configFile = "config.json"
)

type Config struct {
	SessionSecret string `json:"session_secret"`
	CryptoDBPath  string `json:"crypto_db_path"`
	PickleKey     string `json:"pickle_key"`
}

func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(configDir, appName)
	path := filepath.Join(appDir, configFile)

	data, err := os.ReadFile(path)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		applyEnvOverrides(&cfg)
		return &cfg, nil
	}

	secret := make([]byte, 32)
	pickle := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	if _, err := rand.Read(pickle); err != nil {
		return nil, err
	}

	cfg := &Config{
		SessionSecret: base64.StdEncoding.EncodeToString(secret),
		CryptoDBPath:  filepath.Join(appDir, "crypto"),
		PickleKey:     base64.StdEncoding.EncodeToString(pickle),
	}

	if err := os.MkdirAll(appDir, 0700); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		return nil, err
	}

	log.Printf("Generated new config at: %s", path)
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SESSION_SECRET"); v != "" {
		cfg.SessionSecret = v
	}
	if v := os.Getenv("CRYPTO_DB_PATH"); v != "" {
		cfg.CryptoDBPath = v
	}
	if v := os.Getenv("PICKLE_KEY"); v != "" {
		cfg.PickleKey = v
	}
}
