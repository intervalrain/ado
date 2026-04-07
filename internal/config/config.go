package config

import (
	"fmt"
	"os"
"github.com/joho/godotenv"
)

type Config struct {
	Org      string
	Project  string
	PAT      string
	QueryID  string
	Assignee string
	Team     string
	envPath  string
}

// EnvPath returns the .env file path used by this config.
func (c *Config) EnvPath() string {
	return c.envPath
}

func Load() (*Config, error) {
	// ADO_ENV overrides the default .env location (useful for aliases)
	envPath := os.Getenv("ADO_ENV")
	if envPath != "" {
		_ = godotenv.Load(envPath)
	} else {
		envPath = ".env"
		_ = godotenv.Load()
	}

	cfg := &Config{
		Org:      os.Getenv("ADO_ORG"),
		Project:  os.Getenv("ADO_PROJECT"),
		PAT:      os.Getenv("ADO_PAT"),
		QueryID:  os.Getenv("ADO_QUERY_ID"),
		Assignee: os.Getenv("ADO_ASSIGNEE"),
		Team:     os.Getenv("ADO_TEAM"),
		envPath:  envPath,
	}

	if cfg.Org == "" || cfg.PAT == "" {
		return nil, fmt.Errorf("ADO_ORG and ADO_PAT are required, check your .env file")
	}

	return cfg, nil
}
