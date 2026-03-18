package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Config holds all proxy settings. Use DefaultConfig() for production.
type Config struct {
	ListenPort int
	AdminAddr  string
	Dir        string // directory for config, pid, logs
	Secure     bool   // enable HTTPS
}

// DefaultConfig returns the production config using ~/.devup/.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ListenPort: 9909,
		AdminAddr:  "localhost:2019",
		Dir:        filepath.Join(home, ".devup"),
	}
}

func (c Config) adminURL() string  { return "http://" + c.AdminAddr }
func (c Config) logPath() string   { return filepath.Join(c.Dir, "log", "proxy.log") }
func (c Config) configPath() string { return filepath.Join(c.Dir, "caddy.json") }

func (c Config) ensureDir() error {
	return os.MkdirAll(filepath.Join(c.Dir, "log"), 0o755)
}

// NewClient creates a Client for this config.
func (c Config) NewClient() *Client {
	return NewClientWithAddr(c.adminURL())
}

// EnsureRunning starts Caddy if it's not already running.
func (c Config) EnsureRunning() error {
	if err := c.ensureDir(); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Check if caddy admin API is already responding
	if c.isReady() {
		return nil
	}

	caddyPath, err := exec.LookPath("caddy")
	if err != nil {
		return fmt.Errorf("caddy not found in PATH: %w", err)
	}

	// Stop any existing broken caddy (ignore errors)
	exec.Command(caddyPath, "stop").Run()

	accessLog := filepath.Join(c.Dir, "log", "access.log")
	errorLog := filepath.Join(c.Dir, "log", "error.log")

	httpsLine := `"automatic_https": {"disable": true},`
	if c.Secure {
		httpsLine = ""
	}

	config := fmt.Sprintf(`{
  "admin": {"listen": %q},
  "storage": {"module": "file_system", "root": %q},
  "logging": {
    "logs": {
      "error": {
        "writer": {"output": "file", "filename": %q, "roll": true, "roll_size_mb": 10, "roll_keep": 3},
        "level": "ERROR"
      },
      "access": {
        "writer": {"output": "file", "filename": %q, "roll": true, "roll_size_mb": 10, "roll_keep": 3},
        "include": ["http.log.access"]
      }
    }
  },
  "apps": {
    "http": {
      "servers": {
        "devup": {
          "listen": [":%d"],
          %s
          "logs": {"default_logger_name": "access"},
          "routes": []
        }
      }
    }
  }
}`, c.AdminAddr, filepath.Join(c.Dir, "caddy-data"), errorLog, accessLog, c.ListenPort, httpsLine)

	if err := os.WriteFile(c.configPath(), []byte(config), 0o644); err != nil {
		return fmt.Errorf("write caddy config: %w", err)
	}

	// Use "caddy start" which daemonizes itself properly
	logFile, err := os.OpenFile(c.logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(caddyPath, "start", "--config", c.configPath())
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}

	// Wait for admin API to be ready
	for range 20 {
		time.Sleep(100 * time.Millisecond)
		if c.isReady() {
			return nil
		}
	}
	return nil
}

// Stop kills the running Caddy process.
func (c Config) Stop() error {
	caddyPath, err := exec.LookPath("caddy")
	if err != nil {
		return nil
	}
	return exec.Command(caddyPath, "stop").Run()
}

// CheckTrust verifies the Caddy local CA is in the system trust store.
func CheckTrust() error {
	out, err := exec.Command("security", "find-certificate", "-a", "-c", "Caddy", "/Library/Keychains/System.keychain").Output()
	if err != nil || len(out) == 0 {
		return fmt.Errorf("Caddy's local CA certificate is not trusted by your system.\n\nRun this once to fix it:\n\n  caddy trust\n")
	}
	return nil
}

// isReady checks if caddy's admin API is responding.
func (c Config) isReady() bool {
	client := c.NewClient()
	_, err := client.ListRoutes()
	return err == nil
}
