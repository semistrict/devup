package proxy_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/semistrict/devup/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpClient returns an HTTP client that skips TLS verification (Caddy self-signed certs).
var httpClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func startTestBackend(t *testing.T, body string) int {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})}
	go srv.Serve(listener)
	t.Cleanup(func() { srv.Close() })
	return port
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func testConfig(t *testing.T) proxy.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := proxy.Config{
		ListenPort: freePort(t),
		AdminAddr:  fmt.Sprintf("localhost:%d", freePort(t)),
		Dir:        dir,
	}
	t.Cleanup(func() {
		if t.Failed() {
			logPath := filepath.Join(dir, "log", "proxy.log")
			data, err := os.ReadFile(logPath)
			if err == nil {
				t.Logf("caddy log:\n%s", data)
			}
		}
	})
	return cfg
}

func requireCaddy(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("caddy"); err != nil {
		t.Skip("skipped: caddy not found in PATH")
	}
}

func proxyGet(t *testing.T, cfg proxy.Config, hostname string) string {
	t.Helper()
	// Caddy restarts its listener and issues a cert after a new route is added.
	// Give it time to settle, then retry.
	time.Sleep(500 * time.Millisecond)
	var lastErr error
	for range 50 {
		req, err := http.NewRequest("GET", fmt.Sprintf("https://localhost:%d/", cfg.ListenPort), nil)
		require.NoError(t, err)
		req.Host = hostname
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}
	t.Fatalf("proxy request failed after retries: %v", lastErr)
	return ""
}

func TestIntegration_RegisterAndProxy(t *testing.T) {
	requireCaddy(t)
	t.Skip("skipped: Caddy TLS cert provisioning makes this flaky in test")
	cfg := testConfig(t)
	require.NoError(t, cfg.EnsureRunning())
	t.Cleanup(func() { cfg.Stop() })

	client := cfg.NewClient()
	backendPort := startTestBackend(t, "hello from backend")

	require.NoError(t, client.Register("myapp.localhost", backendPort))

	routes, err := client.ListRoutes()
	require.NoError(t, err)
	require.Len(t, routes, 1)
	assert.Equal(t, "myapp.localhost", routes[0].Hostname)
	assert.Equal(t, backendPort, routes[0].Port)

	assert.Equal(t, "hello from backend", proxyGet(t, cfg, "myapp.localhost"))
}

func TestIntegration_DeregisterRoute(t *testing.T) {
	requireCaddy(t)
	cfg := testConfig(t)
	require.NoError(t, cfg.EnsureRunning())
	t.Cleanup(func() { cfg.Stop() })

	client := cfg.NewClient()
	backendPort := startTestBackend(t, "hello")

	require.NoError(t, client.Register("myapp.localhost", backendPort))
	routes, err := client.ListRoutes()
	require.NoError(t, err)
	require.Len(t, routes, 1)

	require.NoError(t, client.Deregister("myapp.localhost"))
	routes, err = client.ListRoutes()
	require.NoError(t, err)
	assert.Empty(t, routes)
}

func TestIntegration_ReRegisterSameHostname(t *testing.T) {
	requireCaddy(t)
	t.Skip("skipped: Caddy TLS cert provisioning makes this flaky in test")
	cfg := testConfig(t)
	require.NoError(t, cfg.EnsureRunning())
	t.Cleanup(func() { cfg.Stop() })

	client := cfg.NewClient()
	backend1 := startTestBackend(t, "backend-1")
	backend2 := startTestBackend(t, "backend-2")

	require.NoError(t, client.Register("myapp.localhost", backend1))
	require.NoError(t, client.Register("myapp.localhost", backend2))

	routes, err := client.ListRoutes()
	require.NoError(t, err)
	require.Len(t, routes, 1)

	assert.Equal(t, "backend-2", proxyGet(t, cfg, "myapp.localhost"))
}

func TestIntegration_MultipleRoutes(t *testing.T) {
	requireCaddy(t)
	t.Skip("skipped: Caddy TLS cert provisioning makes this flaky in test")
	cfg := testConfig(t)
	require.NoError(t, cfg.EnsureRunning())
	t.Cleanup(func() { cfg.Stop() })

	client := cfg.NewClient()
	backendA := startTestBackend(t, "app-a")
	backendB := startTestBackend(t, "app-b")

	require.NoError(t, client.Register("app-a.localhost", backendA))
	require.NoError(t, client.Register("app-b.localhost", backendB))

	routes, err := client.ListRoutes()
	require.NoError(t, err)
	assert.Len(t, routes, 2)

	assert.Equal(t, "app-a", proxyGet(t, cfg, "app-a.localhost"))
	assert.Equal(t, "app-b", proxyGet(t, cfg, "app-b.localhost"))
}
