package envfile

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadLocalEnv reads .env.local from the project root if it exists.
func LoadLocalEnv(projectRoot string) (map[string]string, error) {
	path := filepath.Join(projectRoot, ".env.local")
	values, err := godotenv.Read(path)
	if err == nil {
		return values, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	return nil, err
}
