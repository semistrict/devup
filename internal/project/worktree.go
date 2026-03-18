package project

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectWorktree checks if the given project root is inside a git worktree.
// Returns the worktree name if it is, or empty string if not.
func DetectWorktree(projectRoot string) string {
	gitPath := filepath.Join(projectRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	// Normal git repo has .git as a directory — not a worktree
	if info.IsDir() {
		return ""
	}

	// Worktrees have .git as a file containing: gitdir: /path/to/.git/worktrees/<name>
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return ""
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	// Expected format: <something>/.git/worktrees/<worktree-name>
	parts := strings.Split(filepath.ToSlash(gitdir), "/")
	for i, part := range parts {
		if part == "worktrees" && i >= 1 && parts[i-1] == ".git" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// Hostname returns the hostname for the dev server.
// Format: <worktree>-<project>.devup.localhost or <project>.devup.localhost
func Hostname(projectName, worktreeName string) string {
	if worktreeName != "" {
		return worktreeName + "-" + projectName + ".devup.localhost"
	}
	return projectName + ".devup.localhost"
}
