package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Info holds the detected project name and root directory.
type Info struct {
	Name string
	Root string
}

// Detect walks up from startDir looking for project config files
// and returns the project name and root directory.
func Detect(startDir string) (Info, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Info{}, err
	}

	for {
		if name, ok := detectPackageJSON(dir); ok {
			return Info{Name: name, Root: dir}, nil
		}
		if name, ok := detectCargoToml(dir); ok {
			return Info{Name: name, Root: dir}, nil
		}
		if name, ok := detectPyprojectToml(dir); ok {
			return Info{Name: name, Root: dir}, nil
		}
		if name, ok := detectGoMod(dir); ok {
			return Info{Name: name, Root: dir}, nil
		}
		if isGitRoot(dir) {
			return Info{Name: filepath.Base(dir), Root: dir}, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, use starting dir name
			return Info{Name: filepath.Base(startDir), Root: startDir}, nil
		}
		dir = parent
	}
}

func detectPackageJSON(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil || pkg.Name == "" {
		return "", false
	}
	// Turn @scope/name into scope-name
	name := pkg.Name
	name = strings.TrimPrefix(name, "@")
	name = strings.ReplaceAll(name, "/", "-")
	return name, true
}

func detectCargoToml(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return "", false
	}
	// Simple line-based parsing to avoid TOML dependency
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"'`)
				if name != "" {
					return name, true
				}
			}
		}
	}
	return "", false
}

func detectPyprojectToml(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, `"'`)
				if name != "" {
					return name, true
				}
			}
		}
	}
	return "", false
}

func detectGoMod(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimPrefix(line, "module ")
			mod = strings.TrimSpace(mod)
			// Use last segment of module path
			if i := strings.LastIndex(mod, "/"); i >= 0 {
				mod = mod[i+1:]
			}
			if mod != "" {
				return mod, true
			}
		}
	}
	return "", false
}

func isGitRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	// .git can be a directory (normal repo) or a file (worktree)
	return info.IsDir() || info.Mode().IsRegular()
}
