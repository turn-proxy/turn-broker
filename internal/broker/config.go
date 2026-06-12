package broker

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Bind string   `json:"bind"`
	VK   VKConfig `json:"vk"`
}

type VKConfig struct {
	CookiesFile       string  `json:"cookies_file"`
	AppID             *string `json:"app_id"`
	APIVersion        *string `json:"api_version"`
	SessionRefreshSec *uint64 `json:"session_refresh_secs"`
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Bind == "" {
		return Config{}, fmt.Errorf("bind is required")
	}
	if cfg.VK.CookiesFile == "" {
		return Config{}, fmt.Errorf("vk.cookies_file is required")
	}
	return cfg, nil
}
