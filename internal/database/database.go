package database

import "fmt"

type Config struct {
	DSN string
}

func ValidateConfig(cfg Config) error {
	if cfg.DSN == "" {
		return fmt.Errorf("dsn must not be empty")
	}
	return nil
}
