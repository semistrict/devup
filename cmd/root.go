package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/semistrict/devup/internal/project"
	"github.com/semistrict/devup/internal/proxy"
	"github.com/semistrict/devup/internal/server"
)

type proxyClient interface {
	RegisterDial(hostname string, dial string) error
	Deregister(hostname string) error
	ListRoutes() ([]proxy.Route, error)
}

type cliOptions struct {
	secure bool
	help   bool
}

func Execute() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(args []string) error {
	opts, remaining, err := parseCLIArgs(args)
	if err != nil {
		return err
	}

	if opts.help {
		printUsage(os.Stdout)
		return nil
	}
	if len(remaining) == 0 {
		printUsage(os.Stderr)
		return errors.New("devup: missing command")
	}

	switch remaining[0] {
	case "status":
		if len(remaining) != 1 {
			return fmt.Errorf("devup status: unexpected arguments: %s", strings.Join(remaining[1:], " "))
		}
		return runStatus()
	case "url":
		if len(remaining) != 1 {
			return fmt.Errorf("devup url: unexpected arguments: %s", strings.Join(remaining[1:], " "))
		}
		return runURL(opts.secure)
	default:
		return runRoot(remaining, opts.secure)
	}
}

func parseCLIArgs(args []string) (cliOptions, []string, error) {
	var opts cliOptions

	for i, arg := range args {
		if arg == "--" {
			return opts, args[i+1:], nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return opts, args[i:], nil
		}

		switch arg {
		case "-s", "--secure":
			opts.secure = true
		case "-h", "--help":
			opts.help = true
		default:
			return cliOptions{}, nil, fmt.Errorf("devup: unknown flag %q", arg)
		}
	}

	return opts, nil, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  devup [-s|--secure] <command...>")
	fmt.Fprintln(w, "  devup [-s|--secure] status")
	fmt.Fprintln(w, "  devup [-s|--secure] url")
}

func runRoot(args []string, secure bool) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if os.Getenv("DEVUP") == "1" {
		return fmt.Errorf("devup: refusing to run nested (DEVUP=1 is set)")
	}
	os.Setenv("DEVUP", "1")

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	proj, err := project.Detect(cwd)
	if err != nil {
		return fmt.Errorf("detect project: %w", err)
	}

	worktree := project.DetectWorktree(proj.Root)
	hostname := project.Hostname(proj.Name, worktree)

	if err := server.EnsureDirs(proj.Root); err != nil {
		return err
	}

	serverName := server.Name(args)
	var old *server.PidInfo
	if existing, err := server.ReadPid(proj.Root, serverName); err == nil {
		old = &existing
	}

	cfg := proxy.DefaultConfig()
	cfg.Secure = secure

	if cfg.Secure {
		if err := proxy.CheckTrust(); err != nil {
			return err
		}
	}

	if err := cfg.EnsureRunning(); err != nil {
		logger.Warn("failed to start proxy", "error", err)
	}

	client := cfg.NewClient()

	port, err := server.FindFreePort()
	if err != nil {
		return err
	}

	logPath := server.LogPath(proj.Root, serverName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	output := io.MultiWriter(logFile, os.Stdout)
	var proc *server.Process
	var procMu sync.Mutex
	exitCh := make(chan error, 1)

	cleanup := func() {
		stopServer(&proc, &procMu)
		cleanupRegistration(cfg, client, hostname, proj.Root, serverName, port)
	}
	defer cleanup()

	if err := startServer(logger, args, hostname, port, output, client, &proc, &procMu, proj.Root, serverName,
		old,
		func() {
			scheme := "http"
			if cfg.Secure {
				scheme = "https"
			}
			fmt.Fprintln(os.Stdout, formatStatusLine(fmt.Sprintf("devup: ready %s://%s:%d", scheme, hostname, cfg.ListenPort)))
			fmt.Fprintln(os.Stdout, formatStatusLine(fmt.Sprintf("devup: log %s", displayPath(logPath))))
		},
		func(err error) {
			fmt.Fprintf(os.Stdout, "devup: warning: %v\n", err)
		},
		func(err error) {
			exitCh <- err
		},
	); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		stopServer(&proc, &procMu)
		return nil
	case err := <-exitCh:
		if err != nil {
			return err
		}
		return nil
	}
}

func startServer(
	logger *slog.Logger,
	args []string,
	hostname string,
	port int,
	output io.Writer,
	client *proxy.Client,
	proc **server.Process,
	procMu *sync.Mutex,
	projectRoot string,
	serverName string,
	old *server.PidInfo,
	onRegistered func(),
	onWarning func(error),
	onExit func(error),
) error {
	procMu.Lock()
	defer procMu.Unlock()

	p, err := server.StartProcess(args, port, output, output)
	if err != nil {
		return err
	}
	*proc = p

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		dialAddr, err := server.WaitForListen(ctx, port)
		if err != nil {
			onWarning(err)
			return
		}
		if err := client.RegisterDial(hostname, dialAddr); err != nil {
			logger.Warn("failed to register with proxy", "error", err)
			onWarning(err)
			return
		}
		if err := server.WritePid(projectRoot, serverName, server.PidInfo{
			DevupPID: os.Getpid(),
			ChildPID: p.PID(),
		}); err != nil {
			logger.Warn("failed to write pid file", "error", err)
			onWarning(err)
		}
		stopPreviousInstance(logger, old)
		onRegistered()
	}()

	go func() {
		onExit(p.Wait())
	}()

	return nil
}

func stopPreviousInstance(logger *slog.Logger, old *server.PidInfo) {
	if old == nil {
		return
	}
	if old.ChildPID > 0 && server.IsAlive(old.ChildPID) {
		logger.Info("killing existing child process", "pid", old.ChildPID)
		if err := server.KillProcess(old.ChildPID); err != nil {
			logger.Warn("failed to kill existing child process", "pid", old.ChildPID, "error", err)
		}
	}
	if old.DevupPID > 0 && old.DevupPID != os.Getpid() && server.IsAlive(old.DevupPID) {
		logger.Info("killing existing devup instance", "pid", old.DevupPID)
		if err := server.KillProcess(old.DevupPID); err != nil {
			logger.Warn("failed to kill existing devup instance", "pid", old.DevupPID, "error", err)
		}
	}
}

func cleanupRegistration(cfg proxy.Config, client proxyClient, hostname, projectRoot, serverName string, port int) {
	if routeOwnedByPort(client, hostname, port) {
		_ = client.Deregister(hostname)
	}
	if pidOwnedByCurrentProcess(projectRoot, serverName) {
		server.RemovePid(projectRoot, serverName)
	}

	if routes, err := client.ListRoutes(); err == nil && len(routes) == 0 {
		_ = cfg.Stop()
	}
}

func routeOwnedByPort(client proxyClient, hostname string, port int) bool {
	routes, err := client.ListRoutes()
	if err != nil {
		return false
	}
	for _, route := range routes {
		if route.Hostname == hostname && route.Port == port {
			return true
		}
	}
	return false
}

func pidOwnedByCurrentProcess(projectRoot, serverName string) bool {
	info, err := server.ReadPid(projectRoot, serverName)
	if err != nil {
		return false
	}
	return info.DevupPID == os.Getpid()
}

func stopServer(proc **server.Process, procMu *sync.Mutex) {
	procMu.Lock()
	defer procMu.Unlock()
	if *proc != nil {
		(*proc).Stop()
	}
}

func formatStatusLine(line string) string {
	return "\x1b[1;97m" + line + "\x1b[0m"
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~/" + strings.TrimPrefix(path, home+"/")
	}
	return path
}
