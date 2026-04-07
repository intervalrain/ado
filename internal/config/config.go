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
}

func Load() (*Config, error) {
	// ADO_ENV overrides the default .env location (useful for aliases)
	if envFile := os.Getenv("ADO_ENV"); envFile != "" {
		_ = godotenv.Load(envFile)
	} else {
		_ = godotenv.Load() // fallback: cwd/.env
	}

	cfg := &Config{
		Org:      os.Getenv("ADO_ORG"),
		Project:  os.Getenv("ADO_PROJECT"),
		PAT:      os.Getenv("ADO_PAT"),
		QueryID:  os.Getenv("ADO_QUERY_ID"),
		Assignee: os.Getenv("ADO_ASSIGNEE"),
	}

	if cfg.Org == "" || cfg.PAT == "" {
		return nil, fmt.Errorf("ADO_ORG and ADO_PAT are required, check your .env file")
	}

	return cfg, nil
}
