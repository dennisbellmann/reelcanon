package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads the repository root .env when present. Process environment
// variables always take precedence over values in the file.
func LoadDotEnv() error {
	for _, path := range []string{".env", "../.env"} {
		if _, err := os.Stat(path); err == nil {
			return godotenv.Load(path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
