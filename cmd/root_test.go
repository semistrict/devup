package cmd

import (
	"os"
	"testing"

	"github.com/semistrict/devup/internal/proxy"
	"github.com/semistrict/devup/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProxyClient struct {
	routes        []proxy.Route
	deregistered  []string
	listRoutesErr error
}

func (s *stubProxyClient) RegisterDial(hostname string, dial string) error {
	return nil
}

func (s *stubProxyClient) Deregister(hostname string) error {
	s.deregistered = append(s.deregistered, hostname)
	filtered := s.routes[:0]
	for _, route := range s.routes {
		if route.Hostname != hostname {
			filtered = append(filtered, route)
		}
	}
	s.routes = filtered
	return nil
}

func (s *stubProxyClient) ListRoutes() ([]proxy.Route, error) {
	if s.listRoutesErr != nil {
		return nil, s.listRoutesErr
	}
	routes := make([]proxy.Route, len(s.routes))
	copy(routes, s.routes)
	return routes, nil
}

func TestCleanupRegistrationRemovesOwnedRouteAndPid(t *testing.T) {
	t.Setenv("PATH", "")

	root := t.TempDir()
	require.NoError(t, server.EnsureDirs(root))

	serverName := "npm-run-dev"
	require.NoError(t, server.WritePid(root, serverName, server.PidInfo{
		DevupPID: os.Getpid(),
		ChildPID: 123,
	}))

	client := &stubProxyClient{
		routes: []proxy.Route{{Hostname: "app.localhost", Port: 3000}},
	}

	cleanupRegistration(proxy.Config{}, client, "app.localhost", root, serverName, 3000)

	assert.Equal(t, []string{"app.localhost"}, client.deregistered)
	_, err := os.Stat(server.PidPath(root, serverName))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCleanupRegistrationSkipsForeignRouteAndPid(t *testing.T) {
	t.Setenv("PATH", "")

	root := t.TempDir()
	require.NoError(t, server.EnsureDirs(root))

	serverName := "npm-run-dev"
	require.NoError(t, server.WritePid(root, serverName, server.PidInfo{
		DevupPID: os.Getpid() + 1,
		ChildPID: 123,
	}))

	client := &stubProxyClient{
		routes: []proxy.Route{{Hostname: "app.localhost", Port: 4000}},
	}

	cleanupRegistration(proxy.Config{}, client, "app.localhost", root, serverName, 3000)

	assert.Empty(t, client.deregistered)
	_, err := os.Stat(server.PidPath(root, serverName))
	require.NoError(t, err)
}

func TestParseCLIArgs(t *testing.T) {
	t.Run("parses leading secure flag", func(t *testing.T) {
		opts, remaining, err := parseCLIArgs([]string{"-s", "vite", "--host"})
		require.NoError(t, err)
		assert.True(t, opts.secure)
		assert.False(t, opts.help)
		assert.Equal(t, []string{"vite", "--host"}, remaining)
	})

	t.Run("stops parsing at first non-flag", func(t *testing.T) {
		opts, remaining, err := parseCLIArgs([]string{"vite", "-s"})
		require.NoError(t, err)
		assert.False(t, opts.secure)
		assert.Equal(t, []string{"vite", "-s"}, remaining)
	})

	t.Run("parses help flag", func(t *testing.T) {
		opts, remaining, err := parseCLIArgs([]string{"--help"})
		require.NoError(t, err)
		assert.True(t, opts.help)
		assert.Empty(t, remaining)
	})

	t.Run("supports explicit end of flags", func(t *testing.T) {
		opts, remaining, err := parseCLIArgs([]string{"--", "-s"})
		require.NoError(t, err)
		assert.False(t, opts.secure)
		assert.Equal(t, []string{"-s"}, remaining)
	})

	t.Run("rejects unknown flags", func(t *testing.T) {
		_, _, err := parseCLIArgs([]string{"--verbose"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown flag")
	})
}
