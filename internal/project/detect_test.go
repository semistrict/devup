package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetect_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "@scope/my-app"}`), 0o644)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "scope-my-app", info.Name)
	assert.Equal(t, dir, info.Root)
}

func TestDetect_PackageJSON_NoScope(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "simple-app"}`), 0o644)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "simple-app", info.Name)
}

func TestDetect_CargoToml(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"rust-app\"\n"), 0o644)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "rust-app", info.Name)
	assert.Equal(t, dir, info.Root)
}

func TestDetect_PyprojectToml(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"py-app\"\n"), 0o644)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "py-app", info.Name)
}

func TestDetect_GoMod(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/org/cool-tool\n"), 0o644)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "cool-tool", info.Name)
}

func TestDetect_GitDir(t *testing.T) {
	dir := t.TempDir()
	err := os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	require.NoError(t, err)

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(dir), info.Name)
	assert.Equal(t, dir, info.Root)
}

func TestDetect_WalksUp(t *testing.T) {
	root := t.TempDir()
	err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name": "parent-app"}`), 0o644)
	require.NoError(t, err)

	subdir := filepath.Join(root, "src", "components")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	info, err := Detect(subdir)
	require.NoError(t, err)
	assert.Equal(t, "parent-app", info.Name)
	assert.Equal(t, root, info.Root)
}

func TestDetect_PriorityOrder(t *testing.T) {
	dir := t.TempDir()
	// package.json should win over go.mod
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "js-app"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/org/go-app\n"), 0o644))

	info, err := Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "js-app", info.Name)
}
