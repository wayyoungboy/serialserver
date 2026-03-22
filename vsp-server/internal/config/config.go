package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
}

type JWTConfig struct {
	Secret     string `yaml:"secret"`
	ExpireTime int    `yaml:"expire_time"` // hours
}

type LogConfig struct {
	Level string `yaml:"level"`
	Path  string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9000,
			Mode: "debug",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "./data/vsp.db",
		},
		JWT: JWTConfig{
			Secret:     "vsp-secret-key-change-in-production",
			ExpireTime: 24,
		},
		Log: LogConfig{
			Level: "info",
			Path:  "./logs/vsp.log",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil // Return default config if file not found
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override with environment variables
	if v := os.Getenv("VSP_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("VSP_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("VSP_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}

	return cfg, nil
}