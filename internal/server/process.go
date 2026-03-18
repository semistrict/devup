package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// FindFreePort asks the OS for a free TCP port.
func FindFreePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("find free port: %w", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// Process wraps an exec.Cmd with lifecycle management.
type Process struct {
	Cmd  *exec.Cmd
	Port int
}

// StartProcess launches a command with PORT set, directing output to the given writers.
func StartProcess(args []string, port int, stdout, stderr io.Writer) (*Process, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	var cmd *exec.Cmd
	if os.Getenv("DEVUP_DEBUG") == "ports" {
		profile := fmt.Sprintf("(version 1)\n(allow default)\n(deny network-bind)\n(allow network-bind (local tcp \"*:%d\"))\n", port)
		sandboxArgs := []string{"-p", profile}
		sandboxArgs = append(sandboxArgs, args...)
		cmd = exec.Command("sandbox-exec", sandboxArgs...)
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	// Create a new process group so we can kill the whole tree
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &Process{Cmd: cmd, Port: port}, nil
}

// Stop sends SIGTERM to the process group, then SIGKILL after timeout.
func (p *Process) Stop() error {
	if p.Cmd.Process == nil {
		return nil
	}
	// Kill the entire process group
	pgid, err := syscall.Getpgid(p.Cmd.Process.Pid)
	if err != nil {
		return p.Cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

// Wait waits for the process to exit and returns its error.
func (p *Process) Wait() error {
	return p.Cmd.Wait()
}

// PID returns the process ID.
func (p *Process) PID() int {
	if p.Cmd.Process == nil {
		return 0
	}
	return p.Cmd.Process.Pid
}

// WaitForListen polls until something is listening on the given port.
// Returns the dial address (e.g. "[::1]:8080" or "127.0.0.1:8080").
// Tries IPv6 first, then IPv4.
func WaitForListen(ctx context.Context, port int) (string, error) {
	ipv6 := fmt.Sprintf("[::1]:%d", port)
	ipv4 := fmt.Sprintf("127.0.0.1:%d", port)
	dialer := net.Dialer{Timeout: 200 * time.Millisecond}

	for {
		if conn, err := dialer.DialContext(ctx, "tcp", ipv6); err == nil {
			conn.Close()
			return ipv6, nil
		}
		if conn, err := dialer.DialContext(ctx, "tcp", ipv4); err == nil {
			conn.Close()
			return ipv4, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("nothing listening on port %d: %w", port, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
