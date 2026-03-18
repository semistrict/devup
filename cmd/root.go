package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/semistrict/devup/internal/project"
	"github.com/semistrict/devup/internal/proxy"
	"github.com/semistrict/devup/internal/server"
	"github.com/semistrict/devup/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "devup [command...]",
	Short: "Dev server manager for worktrees",
	Long:  "Wraps a dev server command, allocates a port, and makes it available at <worktree>-<project>.localhost:9909",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRoot,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

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

	logger.Info("detected project", "name", proj.Name, "root", proj.Root, "worktree", worktree, "hostname", hostname)

	if err := server.EnsureDirs(proj.Root); err != nil {
		return err
	}

	// Check for existing instance → kill both devup and its child
	serverName := server.Name(args)
	if old, err := server.ReadPid(proj.Root, serverName); err == nil {
		if old.ChildPID > 0 && server.IsAlive(old.ChildPID) {
			logger.Info("killing existing child process", "pid", old.ChildPID)
			_ = server.KillProcess(old.ChildPID)
		}
		if old.DevupPID > 0 && server.IsAlive(old.DevupPID) {
			logger.Info("killing existing devup instance", "pid", old.DevupPID)
			_ = server.KillProcess(old.DevupPID)
		}
	}
	server.RemovePid(proj.Root, serverName)

	cfg := proxy.DefaultConfig()
	cfg.Secure = secureFlag

	if cfg.Secure {
		if err := proxy.CheckTrust(); err != nil {
			return err
		}
	}

	if err := cfg.EnsureRunning(); err != nil {
		logger.Warn("failed to start proxy", "error", err)
	}

	client := cfg.NewClient()
	_ = client.Deregister(hostname)

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

	model := tui.New(hostname, port, cfg.ListenPort, cfg.Secure, logPath, args)
	program := tea.NewProgram(model, tea.WithAltScreen())

	tuiReader, tuiWriter := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(tuiReader)
		for scanner.Scan() {
			program.Send(tui.OutputMsg(scanner.Text()))
		}
	}()

	output := io.MultiWriter(logFile, tuiWriter)

	var proc *server.Process
	var procMu sync.Mutex

	startServer := func() error {
		procMu.Lock()
		defer procMu.Unlock()

		p, err := server.StartProcess(args, port, output, output)
		if err != nil {
			return err
		}
		proc = p

		// Write PID file with both devup and child PIDs
		if err := server.WritePid(proj.Root, serverName, server.PidInfo{
			DevupPID: os.Getpid(),
			ChildPID: proc.PID(),
		}); err != nil {
			logger.Warn("failed to write pid file", "error", err)
		}

		go func() {
			// Wait for server to start listening, then register with Caddy
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			dialAddr, err := server.WaitForListen(ctx, port)
			if err != nil {
				program.Send(tui.OutputMsg(fmt.Sprintf("[warning: %v]", err)))
			} else {
				if err := client.RegisterDial(hostname, dialAddr); err != nil {
					logger.Warn("failed to register with proxy", "error", err)
				}
			}
		}()

		go func() {
			err := p.Wait()
			program.Send(tui.ProcessExitedMsg{Err: err})
		}()

		return nil
	}

	restartServer := func() {
		procMu.Lock()
		if proc != nil {
			proc.Stop()
		}
		procMu.Unlock()

		program.Send(tui.RestartBannerMsg{})
		if err := startServer(); err != nil {
			program.Send(tui.OutputMsg(fmt.Sprintf("[restart failed: %v]", err)))
		}
	}

	if err := startServer(); err != nil {
		tuiWriter.Close()
		return fmt.Errorf("start server: %w", err)
	}

	tui.SetRestartFunc(restartServer)

	// Bubbletea handles SIGTERM natively — it sends QuitMsg and restores the terminal.

	_, err = program.Run()

	// Cleanup — close pipe first so child gets SIGPIPE, then stop process
	tuiWriter.Close()
	procMu.Lock()
	if proc != nil {
		proc.Stop()
	}
	procMu.Unlock()
	_ = client.Deregister(hostname)
	server.RemovePid(proj.Root, serverName)

	// Stop caddy if we were the last route
	if routes, err := client.ListRoutes(); err == nil && len(routes) == 0 {
		cfg.Stop()
	}

	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

var secureFlag bool

func init() {
	rootCmd.AddCommand(statusCmd)

	rootCmd.Flags().BoolVarP(&secureFlag, "secure", "s", false, "Enable HTTPS (requires `caddy trust` to be run once)")
	rootCmd.Flags().SetInterspersed(false)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

