package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	assert.Equal(t, "vite-dev", Name([]string{"vite", "dev"}))
	assert.Equal(t, "npm-run-start", Name([]string{"npm", "run", "start"}))
	assert.Equal(t, "python", Name([]string{"python"}))
}

func TestEnsureDirs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, EnsureDirs(root))

	info, err := os.Stat(filepath.Join(root, ".devup", "log"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestIsAlive_CurrentProcess(t *testing.T) {
	assert.True(t, IsAlive(os.Getpid()))
}

func TestIsAlive_DeadProcess(t *testing.T) {
	assert.False(t, IsAlive(999999999))
}

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort()
	require.NoError(t, err)
	assert.Greater(t, port, 0)
	assert.Less(t, port, 65536)
}
