package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/semistrict/devup/internal/project"
	"github.com/semistrict/devup/internal/proxy"
)

func runOpen(secure bool) error {
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

	client := proxy.NewClient()
	routes, err := client.ListRoutes()
	if err != nil {
		return fmt.Errorf("devup is not running for %s", proj.Name)
	}

	for _, r := range routes {
		if r.Hostname == hostname {
			url := publicURL(hostname, secure)
			return exec.Command("open", url).Run()
		}
	}

	return fmt.Errorf("devup is not running for %s", proj.Name)
}
