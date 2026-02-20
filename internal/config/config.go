package config

import (
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
	CryptoDBPath string `json:"crypto_db_path"`
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

	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CRYPTO_DB_PATH"); v != "" {
		cfg.CryptoDBPath = v
	}
}
