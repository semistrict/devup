package cmd

import (
	"fmt"
	"os"

	"github.com/semistrict/devup/internal/project"
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
	fmt.Println(publicURL(hostname, secure))
	return nil
}
