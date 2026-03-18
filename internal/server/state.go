package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Name derives a server name from the command args (e.g., ["vite", "dev"] → "vite-dev").
func Name(cmd []string) string {
	return strings.Join(cmd, "-")
}

// DevupDir returns the .devup directory path for the given project root.
func DevupDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".devup")
}

// LogDir returns the .devup/log directory path.
func LogDir(projectRoot string) string {
	return filepath.Join(DevupDir(projectRoot), "log")
}

// EnsureDirs creates the .devup/log directory structure.
func EnsureDirs(projectRoot string) error {
	if err := os.MkdirAll(LogDir(projectRoot), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	return nil
}

// LogPath returns the path to a server's log file.
func LogPath(projectRoot, name string) string {
	return filepath.Join(LogDir(projectRoot), name+".log")
}

// PidPath returns the path to a server's PID file.
func PidPath(projectRoot, name string) string {
	return filepath.Join(DevupDir(projectRoot), name+".pid")
}

// PidInfo holds the devup PID and child process PID.
type PidInfo struct {
	DevupPID int
	ChildPID int
}

// WritePid writes the devup PID to a file. ChildPID can be added later.
func WritePid(projectRoot, name string, info PidInfo) error {
	data := fmt.Sprintf("%d\n%d", info.DevupPID, info.ChildPID)
	return os.WriteFile(PidPath(projectRoot, name), []byte(data), 0o644)
}

// ReadPid reads PIDs from a server's PID file.
func ReadPid(projectRoot, name string) (PidInfo, error) {
	data, err := os.ReadFile(PidPath(projectRoot, name))
	if err != nil {
		return PidInfo{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var info PidInfo
	if len(lines) >= 1 {
		fmt.Sscanf(lines[0], "%d", &info.DevupPID)
	}
	if len(lines) >= 2 {
		fmt.Sscanf(lines[1], "%d", &info.ChildPID)
	}
	return info, nil
}

// RemovePid removes a server's PID file.
func RemovePid(projectRoot, name string) {
	os.Remove(PidPath(projectRoot, name))
}

// IsAlive checks if a process with the given PID is still running.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// KillProcess sends SIGTERM to the given PID, then SIGKILL after timeout.
func KillProcess(pid int) error {
	if !IsAlive(pid) {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for range 600 {
		time.Sleep(100 * time.Millisecond)
		if !IsAlive(pid) {
			return nil
		}
	}
	return process.Signal(syscall.SIGKILL)
}
