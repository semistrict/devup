package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListRoutesParsesDialPort(t *testing.T) {
	routesResponse := []caddyRoute{
		{
			ID:    "app-v4.localhost",
			Match: []caddyMatch{{Host: []string{"app-v4.localhost"}}},
			Handle: []caddyHandler{{
				Handler:   "reverse_proxy",
				Upstreams: []caddyUpstream{{Dial: "127.0.0.1:3000"}},
			}},
		},
		{
			ID:    "app-v6.localhost",
			Match: []caddyMatch{{Host: []string{"app-v6.localhost"}}},
			Handle: []caddyHandler{{
				Handler:   "reverse_proxy",
				Upstreams: []caddyUpstream{{Dial: "[::1]:3001"}},
			}},
		},
		{
			ID:    "app-host.localhost",
			Match: []caddyMatch{{Host: []string{"app-host.localhost"}}},
			Handle: []caddyHandler{{
				Handler:   "reverse_proxy",
				Upstreams: []caddyUpstream{{Dial: "localhost:3002"}},
			}},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/config/apps/http/servers/devup/routes", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode(routesResponse))
	}))
	defer server.Close()

	client := NewClientWithAddr(server.URL)
	routes, err := client.ListRoutes()
	require.NoError(t, err)

	require.Len(t, routes, 3)
	assert.Equal(t, "127.0.0.1:3000", routes[0].Dial)
	assert.Equal(t, 3000, routes[0].Port)
	assert.Equal(t, "[::1]:3001", routes[1].Dial)
	assert.Equal(t, 3001, routes[1].Port)
	assert.Equal(t, "localhost:3002", routes[2].Dial)
	assert.Equal(t, 3002, routes[2].Port)
}
