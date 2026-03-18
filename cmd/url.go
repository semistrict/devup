package cmd

import (
	"fmt"
	"os"

	"github.com/semistrict/devup/internal/project"
	"github.com/semistrict/devup/internal/proxy"
)

func runURL(secure bool) error {
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
	cfg := proxy.DefaultConfig()

	scheme := "http"
	if secure {
		scheme = "https"
	}

	fmt.Printf("%s://%s:%d\n", scheme, hostname, cfg.ListenPort)
	return nil
}
