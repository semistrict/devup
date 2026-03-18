package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectWorktree_NormalRepo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	name := DetectWorktree(dir)
	assert.Equal(t, "", name)
}

func TestDetectWorktree_Worktree(t *testing.T) {
	dir := t.TempDir()
	// Simulate a worktree: .git is a file pointing to the main repo's worktrees dir
	gitContent := "gitdir: /home/user/repo/.git/worktrees/feature-branch"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git"), []byte(gitContent), 0o644))

	name := DetectWorktree(dir)
	assert.Equal(t, "feature-branch", name)
}

func TestDetectWorktree_NoGit(t *testing.T) {
	dir := t.TempDir()

	name := DetectWorktree(dir)
	assert.Equal(t, "", name)
}

func TestHostname_WithWorktree(t *testing.T) {
	h := Hostname("myapp", "feature")
	assert.Equal(t, "feature-myapp.devup.localhost", h)
}

func TestHostname_WithoutWorktree(t *testing.T) {
	h := Hostname("myapp", "")
	assert.Equal(t, "myapp.devup.localhost", h)
}
